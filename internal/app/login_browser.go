package app

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	_ "embed"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/cloudflare"
)

//go:embed login.html
var loginPageHTML string

//go:embed login_done.html
var loginDoneHTML string

type CollectTokenOptions struct {
	NoBrowser bool
	Open      func(string) error
	Stdin     io.Reader
	Stdout    io.Writer
	Timeout   time.Duration
	Listen    string
}

func CollectTokenViaBrowser(ctx context.Context, opt CollectTokenOptions) (string, error) {
	if opt.Open == nil {
		opt.Open = OpenBrowser
	}
	if opt.Stdout == nil {
		opt.Stdout = os.Stderr
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 10 * time.Minute
	}
	if opt.Listen == "" {
		opt.Listen = "127.0.0.1:0"
	}

	state, err := randomState()
	if err != nil {
		return "", err
	}
	cfURL := cloudflare.UserTokenTemplateURL("cfpen")
	page := strings.ReplaceAll(loginPageHTML, "__CF_URL__", html.EscapeString(cfURL))
	page = strings.ReplaceAll(page, "__STATE__", html.EscapeString(state))

	ln, err := net.Listen("tcp", opt.Listen)
	if err != nil {
		return "", fmt.Errorf("listen for login callback: %w", err)
	}
	defer ln.Close()

	tokenCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, page)
	})
	mux.HandleFunc("/done", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.FormValue("state") != state {
			http.Error(w, "invalid state", http.StatusForbidden)
			return
		}
		tok := strings.TrimSpace(r.FormValue("token"))
		if tok == "" {
			http.Error(w, "token required", http.StatusBadRequest)
			return
		}
		select {
		case tokenCh <- tok:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, loginDoneHTML)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	localURL := "http://" + ln.Addr().String() + "/"
	fmt.Fprintf(opt.Stdout, "opening Cloudflare login...\n")
	fmt.Fprintf(opt.Stdout, "local:      %s\n", localURL)
	fmt.Fprintf(opt.Stdout, "cloudflare: %s\n", cfURL)

	if !opt.NoBrowser {
		if err := opt.Open(localURL); err != nil {
			fmt.Fprintf(opt.Stdout, "could not open browser: %v\nopen the local URL above\n", err)
		}
	}

	if opt.Stdin == nil && stdinIsTerminal() && !opt.NoBrowser {
		opt.Stdin = os.Stdin
		fmt.Fprintln(opt.Stdout, "paste the API token here and press Enter, or submit it in the browser page.")
	} else if opt.NoBrowser && opt.Stdin == nil {
		opt.Stdin = os.Stdin
		fmt.Fprintln(opt.Stdout, "paste the API token and press Enter:")
	}

	ctx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()

	if opt.Stdin != nil {
		go func() {
			sc := newLineScanner(opt.Stdin)
			if !sc.Scan() {
				return
			}
			tok := strings.TrimSpace(sc.Text())
			if tok == "" {
				return
			}
			select {
			case tokenCh <- tok:
			case <-ctx.Done():
			}
		}()
	}

	select {
	case tok := <-tokenCh:
		if tok == "" {
			return "", fmt.Errorf("empty token")
		}
		return tok, nil
	case <-ctx.Done():
		return "", fmt.Errorf("login timed out; create a token at %s", cfURL)
	}
}

func newLineScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024), 8<<10)
	return sc
}

func randomState() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
