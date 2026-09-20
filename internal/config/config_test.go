package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOrigin(t *testing.T) {
	tests := []struct {
		raw, proto, want string
	}{
		{"8080", "", "http://127.0.0.1:8080"},
		{"localhost:3000", "http", "http://localhost:3000"},
		{"https://example.com:443", "", "https://example.com:443"},
		{"22", "ssh", "ssh://127.0.0.1:22"},
		{"3389", "rdp", "rdp://127.0.0.1:3389"},
		{"445", "tcp", "tcp://127.0.0.1:445"},
	}
	for _, tc := range tests {
		got, err := ParseOrigin(tc.raw, tc.proto)
		if err != nil {
			t.Fatalf("ParseOrigin(%q, %q): %v", tc.raw, tc.proto, err)
		}
		if got != tc.want {
			t.Fatalf("ParseOrigin(%q, %q)=%q want %q", tc.raw, tc.proto, got, tc.want)
		}
	}
	if _, err := ParseOrigin("", ""); err == nil {
		t.Fatal("expected error for empty origin")
	}
	if _, err := ParseOrigin("80", "ftp"); err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
}

func TestPublicURLAndSuggestName(t *testing.T) {
	if got := PublicURL("app.example.com"); got != "https://app.example.com" {
		t.Fatalf("PublicURL: %s", got)
	}
	if got := PublicURL("https://x.test"); got != "https://x.test" {
		t.Fatalf("PublicURL passthrough: %s", got)
	}
	if got := SuggestServiceName("web.example.com", "http://127.0.0.1:8080"); got != "web" {
		t.Fatalf("SuggestServiceName hostname: %s", got)
	}
	if got := SuggestServiceName("", "http://127.0.0.1:3000"); got != "port-3000" {
		t.Fatalf("SuggestServiceName origin: %s", got)
	}
}

func TestProxyDefaultsAndUpsert(t *testing.T) {
	cfg := Defaults()
	if cfg.Proxy.Listen != DefaultProxyListen {
		t.Fatalf("listen default %s", cfg.Proxy.Listen)
	}
	cfg.ZoneName = "example.com"
	if got := cfg.ProxyHostnameOrDefault(); got != "proxy.example.com" {
		t.Fatalf("proxy hostname %s", got)
	}
	if got := cfg.ProxyOrigin(); got != "tcp://127.0.0.1:41080" {
		t.Fatalf("proxy origin %s", got)
	}
	cfg.UpsertService(Service{Name: "web", Hostname: "web.example.com", Service: "http://127.0.0.1:80"})
	cfg.UpsertService(Service{Name: "web", Hostname: "web.example.com", Service: "http://127.0.0.1:81"})
	if len(cfg.Services) != 1 || cfg.Services[0].Service != "http://127.0.0.1:81" {
		t.Fatalf("upsert: %+v", cfg.Services)
	}
	if !cfg.RemoveService("web") || len(cfg.Services) != 0 {
		t.Fatal("remove failed")
	}
}

func TestClientBundleRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.json")
	in := &ClientBundle{Hostname: "proxy.example.com", ClientID: "abc.access", ClientSecret: "secret", Listen: "127.0.0.1:1080"}
	if err := SaveClientBundle(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadClientBundle(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.Hostname != in.Hostname || out.ClientID != in.ClientID || out.ClientSecret != in.ClientSecret {
		t.Fatalf("bundle %+v", out)
	}

	credsPath := filepath.Join(dir, "credentials.json")
	creds := &Credentials{
		APIToken:       "tok",
		ProxyHostname:  "proxy.example.com",
		AccessClientID: "abc.access",
		AccessSecret:   "secret",
	}
	if err := SaveCredentials(credsPath, creds); err != nil {
		t.Fatal(err)
	}
	fromCreds, err := LoadClientBundle(credsPath)
	if err != nil {
		t.Fatal(err)
	}
	if fromCreds.Hostname != "proxy.example.com" || fromCreds.ClientID != "abc.access" {
		t.Fatalf("from creds %+v", fromCreds)
	}
	_ = os.Remove(path)
}
