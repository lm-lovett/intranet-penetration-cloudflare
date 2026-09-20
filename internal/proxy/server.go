package proxy

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Addr    string
	ln      net.Listener
	mu      sync.Mutex
	closed  bool
	dial    func(network, address string) (net.Conn, error)
	timeout time.Duration
}

func New(addr string) *Server {
	return &Server{
		Addr:    addr,
		timeout: 15 * time.Second,
		dial:    (&net.Dialer{Timeout: 15 * time.Second}).Dial,
	}
}

func (s *Server) ListenAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Addr().String()
	}
	return s.Addr
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ln = ln
	s.closed = false
	s.mu.Unlock()
	go s.serve(ln)
	return nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

func (s *Server) serve(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.timeout))
	br := bufio.NewReader(conn)
	b, err := br.Peek(1)
	if err != nil {
		return
	}
	switch b[0] {
	case 0x05:
		s.handleSOCKS5(conn, br)
	default:
		s.handleHTTP(conn, br)
	}
}

func (s *Server) handleSOCKS5(conn net.Conn, br *bufio.Reader) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(br, header); err != nil {
		return
	}
	nmethods := int(header[1])
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(br, methods); err != nil {
		return
	}
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}
	req := make([]byte, 4)
	if _, err := io.ReadFull(br, req); err != nil {
		return
	}
	if req[0] != 0x05 {
		return
	}
	if req[1] != 0x01 { // CONNECT only
		_, _ = conn.Write(socksReply(0x07, nil, 0))
		return
	}
	host, err := readSOCKSAddr(br, req[3])
	if err != nil {
		_, _ = conn.Write(socksReply(0x01, nil, 0))
		return
	}
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(br, portBuf); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(portBuf)
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	_ = conn.SetDeadline(time.Time{})
	remote, err := s.dial("tcp", target)
	if err != nil {
		_, _ = conn.Write(socksReply(0x05, nil, 0))
		return
	}
	defer remote.Close()
	if _, err := conn.Write(socksReply(0x00, net.ParseIP("0.0.0.0"), 0)); err != nil {
		return
	}
	relay(conn, remote)
}

func readSOCKSAddr(br *bufio.Reader, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(br, ip); err != nil {
			return "", err
		}
		return net.IP(ip).String(), nil
	case 0x03:
		l, err := br.ReadByte()
		if err != nil {
			return "", err
		}
		host := make([]byte, int(l))
		if _, err := io.ReadFull(br, host); err != nil {
			return "", err
		}
		return string(host), nil
	case 0x04:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(br, ip); err != nil {
			return "", err
		}
		return net.IP(ip).String(), nil
	default:
		return "", fmt.Errorf("unsupported atyp %d", atyp)
	}
}

func socksReply(rep byte, ip net.IP, port uint16) []byte {
	if ip == nil {
		ip = net.IPv4zero
	}
	if v4 := ip.To4(); v4 != nil {
		buf := make([]byte, 10)
		buf[0] = 0x05
		buf[1] = rep
		buf[3] = 0x01
		copy(buf[4:8], v4)
		binary.BigEndian.PutUint16(buf[8:], port)
		return buf
	}
	buf := make([]byte, 22)
	buf[0] = 0x05
	buf[1] = rep
	buf[3] = 0x04
	copy(buf[4:20], ip.To16())
	binary.BigEndian.PutUint16(buf[20:], port)
	return buf
}

func (s *Server) handleHTTP(conn net.Conn, br *bufio.Reader) {
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if !strings.EqualFold(req.Method, http.MethodConnect) {
		host := req.Host
		if host == "" {
			host = req.URL.Host
		}
		if _, _, err := net.SplitHostPort(host); err != nil {
			host = net.JoinHostPort(host, "80")
		}
		_ = conn.SetDeadline(time.Time{})
		remote, err := s.dial("tcp", host)
		if err != nil {
			resp := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
			_, _ = conn.Write([]byte(resp))
			return
		}
		defer remote.Close()
		if err := req.Write(remote); err != nil {
			return
		}
		relay(conn, remote)
		return
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "443")
	}
	_ = conn.SetDeadline(time.Time{})
	remote, err := s.dial("tcp", host)
	if err != nil {
		_, _ = conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"))
		return
	}
	defer remote.Close()
	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}
	relay(conn, remote)
}

func relay(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		_ = closeWrite(a)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		_ = closeWrite(b)
	}()
	wg.Wait()
}

func closeWrite(c net.Conn) error {
	type closeWriter interface {
		CloseWrite() error
	}
	if cw, ok := c.(closeWriter); ok {
		return cw.CloseWrite()
	}
	return nil
}
