package app

import (
	"testing"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
)

func TestIngressRulesIncludesProxyAndCatchAll(t *testing.T) {
	cfg := config.Defaults()
	cfg.ZoneName = "example.com"
	cfg.Services = []config.Service{
		{Name: "web", Hostname: "web.example.com", Service: "http://127.0.0.1:8080"},
	}
	rules := ingressRules(cfg, true)
	if len(rules) != 3 {
		t.Fatalf("len=%d", len(rules))
	}
	if rules[1].Hostname != "proxy.example.com" {
		t.Fatalf("proxy hostname %s", rules[1].Hostname)
	}
	off := ingressRules(cfg, false)
	if len(off) != 2 || off[1].Service != "http_status:404" {
		t.Fatalf("without proxy %+v", off)
	}
}
