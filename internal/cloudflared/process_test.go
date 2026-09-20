package cloudflared

import (
	"strings"
	"testing"
)

func TestTunnelAndAccessArgs(t *testing.T) {
	args := TunnelArgs("127.0.0.1:4099", true)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--no-autoupdate") || !strings.Contains(joined, "--metrics 127.0.0.1:4099") {
		t.Fatalf("tunnel args %v", args)
	}
	if args[len(args)-1] != "run" {
		t.Fatalf("expected run last, got %v", args)
	}
	tcp := AccessTCPArgs("proxy.example.com", "127.0.0.1:1080", "id", "secret")
	if tcp[0] != "access" || tcp[1] != "tcp" {
		t.Fatalf("access args %v", tcp)
	}
}

func TestMetricsURL(t *testing.T) {
	if got := metricsURL("127.0.0.1:4099"); got != "http://127.0.0.1:4099/ready" {
		t.Fatalf("got %s", got)
	}
	if got := metricsURL("http://127.0.0.1:1/"); got != "http://127.0.0.1:1/ready" {
		t.Fatalf("got %s", got)
	}
}
