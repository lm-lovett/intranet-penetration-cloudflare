package dashboard

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"

	_ "embed"
)

//go:embed index.html
var indexHTML string

type Status struct {
	Mode          string `json:"mode"`
	Running       bool   `json:"running"`
	TunnelID      string `json:"tunnel_id,omitempty"`
	ProxyHostname string `json:"proxy_hostname,omitempty"`
	ProxyListen   string `json:"proxy_listen,omitempty"`
	ClientListen  string `json:"client_listen,omitempty"`
	Dashboard     string `json:"dashboard,omitempty"`
}

type Service struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Service  string `json:"service"`
	Path     string `json:"path,omitempty"`
	URL      string `json:"url"`
}

type ExposeRequest struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Origin   string `json:"origin"`
	Protocol string `json:"protocol"`
	Path     string `json:"path"`
}

type Hooks struct {
	Status func() Status
	List   func() []Service
	Expose func(ExposeRequest) error
	Remove func(name string) error
	Client func() (any, error)
}

type Server struct {
	Addr string
	Hook Hooks
	ln   net.Listener
	srv  *http.Server
}

func New(addr string, hook Hooks) *Server {
	return &Server{Addr: addr, Hook: hook}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/services", s.handleServices)
	mux.HandleFunc("/api/services/", s.handleServiceDelete)
	mux.HandleFunc("/api/client", s.handleClient)
	s.srv = &http.Server{Addr: s.Addr, Handler: mux}
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	s.ln = ln
	go func() { _ = s.srv.Serve(ln) }()
	return nil
}

func (s *Server) Close() error {
	if s.srv != nil {
		return s.srv.Close()
	}
	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, indexHTML)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Hook.Status())
}

func (s *Server) handleClient(w http.ResponseWriter, r *http.Request) {
	if s.Hook.Client == nil {
		writeJSON(w, map[string]string{"error": "unavailable"})
		return
	}
	v, err := s.Hook.Client()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, v)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"services": s.Hook.List()})
	case http.MethodPost:
		var req ExposeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.Hook.Expose(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"ok": "1"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleServiceDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/services/")
	if name == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}
	if err := s.Hook.Remove(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"ok": "1"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
