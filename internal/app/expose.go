package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
)

type ExposeOptions struct {
	Origin   string
	Hostname string
	Name     string
	Protocol string
	Path     string
}

func (a *App) Expose(ctx context.Context, opt ExposeOptions) (*config.Service, error) {
	if err := a.RequireZone(); err != nil {
		return nil, err
	}
	origin, err := config.ParseOrigin(opt.Origin, opt.Protocol)
	if err != nil {
		return nil, err
	}
	hostname := strings.TrimSpace(opt.Hostname)
	name := strings.TrimSpace(opt.Name)
	if hostname == "" {
		if name == "" {
			name = config.SuggestServiceName("", origin)
		}
		hostname = name + "." + a.Cfg.ZoneName
	}
	if name == "" {
		name = config.SuggestServiceName(hostname, origin)
	}
	svc := config.Service{
		Name:     name,
		Hostname: hostname,
		Service:  origin,
		Path:     strings.TrimSpace(opt.Path),
	}
	a.Cfg.UpsertService(svc)
	includeProxy := !a.Cfg.Proxy.Disable && a.Creds.AccessClientID != ""
	if err := a.SyncTunnel(ctx, includeProxy); err != nil {
		return nil, err
	}
	fmt.Printf("exposed %s -> %s\n", config.PublicURL(svc.Hostname), svc.Service)
	return &svc, nil
}

func (a *App) Unexpose(ctx context.Context, name string) error {
	_, svc, ok := a.Cfg.FindService(name)
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}
	a.Cfg.RemoveService(name)
	cf, err := a.Client()
	if err != nil {
		return err
	}
	if a.Cfg.ZoneID != "" && svc.Hostname != "" {
		if err := cf.DeleteDNS(ctx, a.Cfg.ZoneID, svc.Hostname); err != nil {
			fmt.Printf("warning: delete dns %s: %v\n", svc.Hostname, err)
		}
	}
	includeProxy := !a.Cfg.Proxy.Disable && a.Creds.AccessClientID != ""
	if err := a.SyncTunnel(ctx, includeProxy); err != nil {
		return err
	}
	fmt.Printf("removed %s\n", svc.Name)
	return nil
}

func (a *App) List() {
	if len(a.Cfg.Services) == 0 {
		fmt.Println("no services")
		return
	}
	for _, s := range a.Cfg.Services {
		fmt.Printf("%-16s %-32s %s\n", s.Name, config.PublicURL(s.Hostname), s.Service)
	}
	host := a.Cfg.ProxyHostnameOrDefault()
	if host != "" && !a.Cfg.Proxy.Disable {
		fmt.Printf("%-16s %-32s %s\n", "(proxy)", host, a.Cfg.ProxyOrigin())
	}
}
