package cloudflare

import (
	"net/url"
	"strings"
	"testing"
)

func TestUserTokenTemplateURL(t *testing.T) {
	raw := UserTokenTemplateURL("cfpen")
	if !strings.HasPrefix(raw, TokenCreateBase) {
		t.Fatalf("prefix: %s", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("name") != "cfpen" || q.Get("accountId") != "*" || q.Get("zoneId") != "all" {
		t.Fatalf("query %+v", q)
	}
	keys := q.Get("permissionGroupKeys")
	for _, need := range []string{"cloudflare_tunnel", "dns", "access", "account_settings"} {
		if !strings.Contains(keys, need) {
			t.Fatalf("missing %s in %s", need, keys)
		}
	}
}
