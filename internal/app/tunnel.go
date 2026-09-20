package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
)

func (a *App) SyncTunnel(ctx context.Context, includeProxy bool) error {
	cf, err := a.Client()
	if err != nil {
		return err
	}
	if err := a.RequireZone(); err != nil {
		return err
	}
	tunnelID := a.Creds.TunnelID
	if tunnelID == "" {
		tun, err := cf.EnsureTunnel(ctx, a.Cfg.TunnelName)
		if err != nil {
			return fmt.Errorf("ensure tunnel: %w", err)
		}
		tunnelID = tun.ID
		a.Creds.TunnelID = tun.ID
		if tun.Token != "" {
			a.Creds.TunnelToken = tun.Token
		}
	}
	if a.Creds.TunnelToken == "" {
		tok, err := cf.GetTunnelToken(ctx, tunnelID)
		if err != nil {
			return fmt.Errorf("tunnel token: %w", err)
		}
		a.Creds.TunnelToken = tok
	}
	rules := ingressRules(a.Cfg, includeProxy)
	if err := cf.SetTunnelConfig(ctx, tunnelID, rules); err != nil {
		return fmt.Errorf("set tunnel config: %w", err)
	}
	for _, svc := range a.Cfg.Services {
		if strings.TrimSpace(svc.Hostname) == "" {
			continue
		}
		if _, err := cf.EnsureDNS(ctx, a.Cfg.ZoneID, svc.Hostname, tunnelID); err != nil {
			return fmt.Errorf("dns %s: %w", svc.Hostname, err)
		}
	}
	if includeProxy && !a.Cfg.Proxy.Disable {
		host := a.Cfg.ProxyHostnameOrDefault()
		if host != "" {
			if _, err := cf.EnsureDNS(ctx, a.Cfg.ZoneID, host, tunnelID); err != nil {
				return fmt.Errorf("dns %s: %w", host, err)
			}
			a.Creds.ProxyHostname = host
			a.Cfg.Proxy.Hostname = host
		}
	}
	return a.Save()
}

func (a *App) EnsureAccess(ctx context.Context) error {
	cf, err := a.Client()
	if err != nil {
		return err
	}
	if _, err := cf.EnsureAccessOrganization(ctx); err != nil {
		return err
	}
	host := a.Cfg.ProxyHostnameOrDefault()
	if host == "" {
		return fmt.Errorf("proxy hostname empty; set zone or proxy.hostname")
	}
	app, err := cf.EnsureAccessApp(ctx, "cfpen-proxy", host)
	if err != nil {
		return fmt.Errorf("access app: %w", err)
	}
	a.Creds.AccessAppID = app.ID
	a.Creds.ProxyHostname = host

	tokenID := ""
	if a.Creds.AccessClientID != "" && a.Creds.AccessSecret != "" {
		tokens, err := cf.ListServiceTokens(ctx)
		if err != nil {
			return err
		}
		for _, t := range tokens {
			if t.ClientID == a.Creds.AccessClientID {
				tokenID = t.ID
				break
			}
		}
	}
	if tokenID == "" || a.Creds.AccessSecret == "" {
		tok, err := cf.CreateServiceToken(ctx, "cfpen-client")
		if err != nil {
			return fmt.Errorf("service token: %w", err)
		}
		tokenID = tok.ID
		a.Creds.AccessClientID = tok.ClientID
		if tok.ClientSecret != "" {
			a.Creds.AccessSecret = tok.ClientSecret
		}
	}
	if err := cf.EnsureServiceAuthPolicy(ctx, app.ID, tokenID); err != nil {
		return fmt.Errorf("access policy: %w", err)
	}
	return a.Save()
}

func (a *App) ClientBundle() *config.ClientBundle {
	listen := a.Cfg.Proxy.ClientListen
	host := a.Cfg.ProxyHostnameOrDefault()
	if a.Creds.ProxyHostname != "" {
		host = a.Creds.ProxyHostname
	}
	return a.Creds.ClientBundle(host, listen)
}
