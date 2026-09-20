package cloudflare

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) GetAccessOrganization(ctx context.Context) (*AccessOrganization, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	var out AccessOrganization
	path := fmt.Sprintf("/accounts/%s/access/organizations", url.PathEscape(accountID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateAccessOrganization(ctx context.Context, name, authDomain string) (*AccessOrganization, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]string{
		"name":        name,
		"auth_domain": authDomain,
	}
	var out AccessOrganization
	path := fmt.Sprintf("/accounts/%s/access/organizations", url.PathEscape(accountID))
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) EnsureAccessOrganization(ctx context.Context) (*AccessOrganization, error) {
	org, err := c.GetAccessOrganization(ctx)
	if err == nil && org != nil && org.AuthDomain != "" {
		return org, nil
	}
	if err != nil && !IsNotFound(err) {
		// Some accounts return 400 until Zero Trust is opened once.
		if e, ok := err.(*Error); ok && e.Status != 400 && e.Status != 404 {
			return nil, err
		}
	}
	accountID, err2 := c.EnsureAccountID(ctx)
	if err2 != nil {
		return nil, err2
	}
	short := accountID
	if len(short) > 8 {
		short = short[:8]
	}
	authDomain := fmt.Sprintf("cfpen-%s.cloudflareaccess.com", strings.ToLower(short))
	org, err = c.CreateAccessOrganization(ctx, "cfpen", authDomain)
	if err != nil {
		return nil, fmt.Errorf("enable Cloudflare Zero Trust first (https://one.dash.cloudflare.com): %w", err)
	}
	return org, nil
}

func (c *Client) ListAccessApps(ctx context.Context) ([]AccessApp, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	var out []AccessApp
	path := fmt.Sprintf("/accounts/%s/access/apps?per_page=100", url.PathEscape(accountID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) FindAccessApp(ctx context.Context, domain string) (*AccessApp, error) {
	apps, err := c.ListAccessApps(ctx)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		if strings.EqualFold(apps[i].Domain, domain) {
			return &apps[i], nil
		}
	}
	return nil, nil
}

func (c *Client) CreateAccessApp(ctx context.Context, name, domain string) (*AccessApp, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"name":             name,
		"domain":           domain,
		"type":             "self_hosted",
		"session_duration": "24h",
	}
	var out AccessApp
	path := fmt.Sprintf("/accounts/%s/access/apps", url.PathEscape(accountID))
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) EnsureAccessApp(ctx context.Context, name, domain string) (*AccessApp, error) {
	existing, err := c.FindAccessApp(ctx, domain)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	return c.CreateAccessApp(ctx, name, domain)
}

func (c *Client) ListServiceTokens(ctx context.Context) ([]ServiceToken, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	var out []ServiceToken
	path := fmt.Sprintf("/accounts/%s/access/service_tokens?per_page=100", url.PathEscape(accountID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateServiceToken(ctx context.Context, name string) (*ServiceToken, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]string{"name": name}
	var out ServiceToken
	path := fmt.Sprintf("/accounts/%s/access/service_tokens", url.PathEscape(accountID))
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListAccessPolicies(ctx context.Context, appID string) ([]AccessPolicy, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	var out []AccessPolicy
	path := fmt.Sprintf("/accounts/%s/access/apps/%s/policies", url.PathEscape(accountID), url.PathEscape(appID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateAccessPolicy(ctx context.Context, appID string, policy AccessPolicy) (*AccessPolicy, error) {
	accountID, err := c.EnsureAccountID(ctx)
	if err != nil {
		return nil, err
	}
	var out AccessPolicy
	path := fmt.Sprintf("/accounts/%s/access/apps/%s/policies", url.PathEscape(accountID), url.PathEscape(appID))
	if err := c.do(ctx, http.MethodPost, path, policy, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) EnsureServiceAuthPolicy(ctx context.Context, appID, tokenID string) error {
	policies, err := c.ListAccessPolicies(ctx, appID)
	if err != nil {
		return err
	}
	for _, p := range policies {
		if strings.EqualFold(p.Decision, "non_identity") || strings.EqualFold(p.Decision, "service_auth") {
			if policyIncludesToken(p, tokenID) {
				return nil
			}
		}
	}
	_, err = c.CreateAccessPolicy(ctx, appID, AccessPolicy{
		Name:     "cfpen service auth",
		Decision: "non_identity",
		Include: []map[string]any{
			{"service_token": map[string]any{"token_id": tokenID}},
		},
	})
	if err != nil {
		_, err = c.CreateAccessPolicy(ctx, appID, AccessPolicy{
			Name:     "cfpen service auth",
			Decision: "service_auth",
			Include: []map[string]any{
				{"service_token": map[string]any{"id": tokenID}},
			},
		})
	}
	return err
}

func policyIncludesToken(p AccessPolicy, tokenID string) bool {
	for _, inc := range p.Include {
		raw, ok := inc["service_token"]
		if !ok {
			continue
		}
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"token_id", "id"} {
			if v, ok := m[key]; ok && fmt.Sprint(v) == tokenID {
				return true
			}
		}
	}
	return false
}
