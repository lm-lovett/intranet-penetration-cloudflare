package cloudflare

import "testing"

func TestPolicyIncludesToken(t *testing.T) {
	p := AccessPolicy{
		Decision: "non_identity",
		Include: []map[string]any{
			{"service_token": map[string]any{"token_id": "abc"}},
		},
	}
	if !policyIncludesToken(p, "abc") {
		t.Fatal("expected match on token_id")
	}
	p.Include = []map[string]any{{"email": map[string]any{"email": "a@b.c"}}}
	if policyIncludesToken(p, "abc") {
		t.Fatal("unexpected match")
	}
}
