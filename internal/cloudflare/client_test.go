package cloudflare

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIngressFromConfig(t *testing.T) {
	rules := IngressFromConfig(
		[]string{"app.example.com"},
		[]string{"http://127.0.0.1:8080"},
		[]string{""},
		"proxy.example.com",
		"tcp://127.0.0.1:41080",
	)
	if len(rules) != 3 {
		t.Fatalf("len=%d", len(rules))
	}
	if rules[0].Hostname != "app.example.com" || rules[0].Service != "http://127.0.0.1:8080" {
		t.Fatalf("http rule %+v", rules[0])
	}
	if rules[1].Hostname != "proxy.example.com" || rules[1].Service != "tcp://127.0.0.1:41080" {
		t.Fatalf("proxy rule %+v", rules[1])
	}
	if rules[2].Service != "http_status:404" {
		t.Fatalf("catchall %+v", rules[2])
	}
}

func TestRulesFromServicesCatchAll(t *testing.T) {
	rules := RulesFromServices([]string{"a.example.com"}, []string{"http://127.0.0.1:1"}, nil)
	if rules[len(rules)-1].Service != "http_status:404" {
		t.Fatal("missing catch-all")
	}
}

func TestClientTunnelAndDNS(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, []Account{{ID: "acc1", Name: "demo"}})
	})
	mux.HandleFunc("/accounts/acc1/cfd_tunnel", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") == "cfpen" && r.Method == http.MethodGet {
			writeOK(w, []Tunnel{})
			return
		}
		if r.Method == http.MethodPost {
			writeOK(w, Tunnel{ID: "tun1", Name: "cfpen", Token: "tkn"})
			return
		}
		writeOK(w, []Tunnel{})
	})
	mux.HandleFunc("/zones/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodGet {
			writeOK(w, []DNSRecord{})
			return
		}
		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodPost {
			var rec DNSRecord
			_ = json.NewDecoder(r.Body).Decode(&rec)
			rec.ID = "dns1"
			writeOK(w, rec)
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New("token", "acc1")
	c.BaseURL = srv.URL
	ctx := context.Background()
	tun, err := c.EnsureTunnel(ctx, "cfpen")
	if err != nil {
		t.Fatal(err)
	}
	if tun.ID != "tun1" {
		t.Fatalf("tunnel %+v", tun)
	}
	rec, err := c.EnsureDNS(ctx, "zone1", "app.example.com", "tun1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Content != "tun1.cfargotunnel.com" {
		t.Fatalf("dns %+v", rec)
	}
}

func TestVerifyTokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"success":false,"errors":[{"message":"invalid token"}]}`)
	}))
	defer srv.Close()
	c := New("bad", "")
	c.BaseURL = srv.URL
	_, err := c.VerifyToken(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("err=%v", err)
	}
}

func writeOK(w http.ResponseWriter, result any) {
	raw, _ := json.Marshal(map[string]any{"success": true, "result": result, "errors": []any{}})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}
