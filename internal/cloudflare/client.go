package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

type Client struct {
	Token      string
	AccountID  string
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

func New(token, accountID string) *Client {
	return &Client{
		Token:     token,
		AccountID: accountID,
		BaseURL:   DefaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent: "cfpen/0.1",
	}
}

func (c *Client) VerifyToken(ctx context.Context) (*TokenVerify, error) {
	var out TokenVerify
	if err := c.do(ctx, http.MethodGet, "/user/tokens/verify", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListAccounts(ctx context.Context) ([]Account, error) {
	var out []Account
	if err := c.do(ctx, http.MethodGet, "/accounts?per_page=50", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) EnsureAccountID(ctx context.Context) (string, error) {
	if c.AccountID != "" {
		return c.AccountID, nil
	}
	accounts, err := c.ListAccounts(ctx)
	if err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("no Cloudflare accounts visible to this token")
	}
	c.AccountID = accounts[0].ID
	return c.AccountID, nil
}

func (c *Client) ListZones(ctx context.Context, name string) ([]Zone, error) {
	q := "/zones?per_page=50"
	if name != "" {
		q += "&name=" + url.QueryEscape(name)
	}
	var out []Zone
	if err := c.do(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) FindZone(ctx context.Context, hostname string) (*Zone, error) {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return nil, fmt.Errorf("empty hostname")
	}
	candidates := zoneCandidates(hostname)
	for _, name := range candidates {
		zones, err := c.ListZones(ctx, name)
		if err != nil {
			return nil, err
		}
		for _, z := range zones {
			if strings.EqualFold(z.Name, name) {
				return &z, nil
			}
		}
	}
	return nil, fmt.Errorf("no Cloudflare zone found for %s", hostname)
}

func (c *Client) ListTunnels(ctx context.Context, name string) ([]Tunnel, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf("/accounts/%s/cfd_tunnel?is_deleted=false&per_page=100", url.PathEscape(accountID))
	if name != "" {
		q += "&name=" + url.QueryEscape(name)
	}
	var out []Tunnel
	if err := c.do(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetTunnel(ctx context.Context, tunnelID string) (*Tunnel, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	var out Tunnel
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", url.PathEscape(accountID), url.PathEscape(tunnelID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateTunnel(ctx context.Context, name string) (*Tunnel, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]string{
		"name":       name,
		"config_src": "cloudflare",
	}
	var out Tunnel
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel", url.PathEscape(accountID))
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteTunnel(ctx context.Context, tunnelID string) error {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", url.PathEscape(accountID), url.PathEscape(tunnelID))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) GetTunnelToken(ctx context.Context, tunnelID string) (string, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/token", url.PathEscape(accountID), url.PathEscape(tunnelID))
	var token string
	if err := c.do(ctx, http.MethodGet, path, nil, &token); err != nil {
		return "", err
	}
	return token, nil
}

func (c *Client) SetTunnelConfig(ctx context.Context, tunnelID string, rules []IngressRule) error {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/configurations", url.PathEscape(accountID), url.PathEscape(tunnelID))
	body := TunnelConfig{Config: IngressConfig{Ingress: rules}}
	return c.do(ctx, http.MethodPut, path, body, nil)
}

func (c *Client) GetTunnelConfig(ctx context.Context, tunnelID string) (*TunnelConfig, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/configurations", url.PathEscape(accountID), url.PathEscape(tunnelID))
	var out TunnelConfig
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) EnsureDNS(ctx context.Context, zoneID, hostname, tunnelID string) (*DNSRecord, error) {
	target := tunnelID + ".cfargotunnel.com"
	existing, err := c.FindDNS(ctx, zoneID, hostname)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if strings.EqualFold(existing.Content, target) && existing.Type == "CNAME" {
			return existing, nil
		}
		existing.Type = "CNAME"
		existing.Content = target
		existing.Proxied = true
		existing.TTL = 1
		existing.Comment = "Managed by cfpen"
		return c.updateDNS(ctx, zoneID, existing)
	}
	rec := DNSRecord{
		Type:    "CNAME",
		Name:    hostname,
		Content: target,
		Proxied: true,
		TTL:     1,
		Comment: "Managed by cfpen",
	}
	path := fmt.Sprintf("/zones/%s/dns_records", url.PathEscape(zoneID))
	var out DNSRecord
	if err := c.do(ctx, http.MethodPost, path, rec, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) FindDNS(ctx context.Context, zoneID, hostname string) (*DNSRecord, error) {
	q := fmt.Sprintf("/zones/%s/dns_records?name=%s&per_page=20", url.PathEscape(zoneID), url.QueryEscape(hostname))
	var out []DNSRecord
	if err := c.do(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

func (c *Client) updateDNS(ctx context.Context, zoneID string, rec *DNSRecord) (*DNSRecord, error) {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(rec.ID))
	var out DNSRecord
	if err := c.do(ctx, http.MethodPatch, path, rec, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteDNS(ctx context.Context, zoneID, hostname string) error {
	rec, err := c.FindDNS(ctx, zoneID, hostname)
	if err != nil {
		return err
	}
	if rec == nil {
		return nil
	}
	path := fmt.Sprintf("/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(rec.ID))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) FindTunnelByName(ctx context.Context, name string) (*Tunnel, error) {
	tunnels, err := c.ListTunnels(ctx, name)
	if err != nil {
		return nil, err
	}
	for i := range tunnels {
		if strings.EqualFold(tunnels[i].Name, name) {
			return &tunnels[i], nil
		}
	}
	return nil, nil
}

func (c *Client) EnsureTunnel(ctx context.Context, name string) (*Tunnel, error) {
	existing, err := c.FindTunnelByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	return c.CreateTunnel(ctx, name)
}

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env Envelope[json.RawMessage]
	if err := json.Unmarshal(raw, &env); err != nil {
		return &Error{Status: resp.StatusCode, Message: truncate(raw, 300)}
	}
	if !env.Success || resp.StatusCode >= 400 {
		msg := "request failed"
		if len(env.Errors) > 0 {
			msg = env.Errors[0].Message
		}
		return &Error{Status: resp.StatusCode, Message: msg}
	}
	if result == nil || len(env.Result) == 0 || string(env.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Result, result); err != nil {
		return fmt.Errorf("decode result: %w", err)
	}
	return nil
}

func zoneCandidates(hostname string) []string {
	hostname = strings.TrimPrefix(hostname, "https://")
	hostname = strings.TrimPrefix(hostname, "http://")
	if i := strings.IndexByte(hostname, '/'); i >= 0 {
		hostname = hostname[:i]
	}
	parts := strings.Split(hostname, ".")
	var out []string
	for i := 0; i < len(parts)-1; i++ {
		out = append(out, strings.Join(parts[i:], "."))
	}
	return out
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

func CatchAllRule() IngressRule {
	return IngressRule{Service: "http_status:404"}
}

func ProxyTCPRule(hostname, origin string) IngressRule {
	return IngressRule{Hostname: hostname, Service: origin}
}

func RulesFromServices(hostnames []string, origins []string, paths []string) []IngressRule {
	rules := make([]IngressRule, 0, len(origins)+1)
	for i, origin := range origins {
		rule := IngressRule{Service: origin}
		if i < len(hostnames) {
			rule.Hostname = hostnames[i]
		}
		if i < len(paths) && paths[i] != "" {
			rule.Path = paths[i]
		}
		rules = append(rules, rule)
	}
	return append(rules, CatchAllRule())
}

func IngressFromConfig(hostnames, origins, paths []string, proxyHostname, proxyOrigin string) []IngressRule {
	rules := make([]IngressRule, 0, len(origins)+2)
	for i, origin := range origins {
		rule := IngressRule{Service: origin}
		if i < len(hostnames) {
			rule.Hostname = hostnames[i]
		}
		if i < len(paths) && paths[i] != "" {
			rule.Path = paths[i]
		}
		rules = append(rules, rule)
	}
	if proxyHostname != "" && proxyOrigin != "" {
		rules = append(rules, ProxyTCPRule(proxyHostname, proxyOrigin))
	}
	return append(rules, CatchAllRule())
}
