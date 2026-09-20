package cloudflare

import (
	"encoding/json"
	"net/url"
)

const TokenCreateBase = "https://dash.cloudflare.com/profile/api-tokens"

// TokenTemplatePermissions pre-fills the dashboard token form for cfpen.
var TokenTemplatePermissions = []map[string]string{
	{"key": "cloudflare_tunnel", "type": "edit"},
	{"key": "dns", "type": "edit"},
	{"key": "access", "type": "edit"},
	{"key": "access_acct", "type": "edit"},
	{"key": "account_settings", "type": "read"},
}

func UserTokenTemplateURL(name string) string {
	if name == "" {
		name = "cfpen"
	}
	raw, _ := json.Marshal(TokenTemplatePermissions)
	q := url.Values{}
	q.Set("permissionGroupKeys", string(raw))
	q.Set("accountId", "*")
	q.Set("zoneId", "all")
	q.Set("name", name)
	return TokenCreateBase + "?" + q.Encode()
}
