package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/cloudflared"
	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/dashboard"
	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/proxy"
)

type UpOptions struct {
	NoProxy     bool
	NoDashboard bool
}

func (a *App) Up(ctx context.Context, opt UpOptions) error {
	if err := a.RequireZone(); err != nil {
		return err
	}
	if pid, err := config.ReadPID(); err == nil && pidAlive(pid) && pid != os.Getpid() {
		return fmt.Errorf("already running (pid %d); run cfpen down first", pid)
	}

	cf, err := a.Client()
	if err != nil {
		return err
	}
	tun, err := cf.EnsureTunnel(ctx, a.Cfg.TunnelName)
	if err != nil {
		return fmt.Errorf("ensure tunnel: %w", err)
	}
	a.Creds.TunnelID = tun.ID
	if tun.Token != "" {
		a.Creds.TunnelToken = tun.Token
	}
	if a.Creds.TunnelToken == "" {
		tok, err := cf.GetTunnelToken(ctx, tun.ID)
		if err != nil {
			return err
		}
		a.Creds.TunnelToken = tok
	}

	includeProxy := !opt.NoProxy && !a.Cfg.Proxy.Disable
	if includeProxy {
		if err := a.EnsureAccess(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: intranet proxy disabled (%v)\n", err)
			includeProxy = false
		}
	}
	if err := a.SyncTunnel(ctx, includeProxy); err != nil {
		return err
	}

	var socks *proxy.Server
	if includeProxy {
		socks = proxy.New(a.Cfg.Proxy.Listen)
		if err := socks.Start(); err != nil {
			return fmt.Errorf("start socks: %w", err)
		}
		defer socks.Close()
		fmt.Printf("intranet socks listening on %s\n", socks.ListenAddr())
	}

	bin, err := cloudflared.EnsureBinary(ctx, a.Cfg.Cloudflared.Bin)
	if err != nil {
		return err
	}
	logFile, err := config.PathInDir("cloudflared.log")
	if err != nil {
		return err
	}
	proc, err := cloudflared.Start(ctx, cloudflared.RunOptions{
		Bin:     bin,
		Args:    cloudflared.TunnelArgs(a.Cfg.Cloudflared.Metrics, a.Cfg.Cloudflared.NoAutoUpdate),
		Env:     []string{"TUNNEL_TOKEN=" + a.Creds.TunnelToken},
		LogFile: logFile,
	})
	if err != nil {
		return fmt.Errorf("start cloudflared: %w", err)
	}
	defer proc.Stop()

	if err := cloudflared.WaitReady(ctx, a.Cfg.Cloudflared.Metrics, 45*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cloudflared not ready yet: %v\n", err)
	}

	var dash *dashboard.Server
	if !opt.NoDashboard && !a.Cfg.Dashboard.Disable {
		dash = dashboard.New(a.Cfg.Dashboard.Listen, dashboard.Hooks{
			Status: func() dashboard.Status {
				_ = a.Reload()
				return a.dashboardStatus(true, includeProxy, proc)
			},
			List: func() []dashboard.Service {
				_ = a.Reload()
				return a.dashboardServices()
			},
			Expose: func(req dashboard.ExposeRequest) error {
				_, err := a.Expose(ctx, ExposeOptions{
					Origin:   req.Origin,
					Hostname: req.Hostname,
					Name:     req.Name,
					Protocol: req.Protocol,
					Path:     req.Path,
				})
				return err
			},
			Remove: func(name string) error {
				return a.Unexpose(ctx, name)
			},
			Client: func() (any, error) {
				b := a.ClientBundle()
				if b.ClientID == "" {
					return b, fmt.Errorf("proxy client credentials not ready")
				}
				return b, nil
			},
		})
		if err := dash.Start(); err != nil {
			return fmt.Errorf("dashboard: %w", err)
		}
		defer dash.Close()
		fmt.Printf("dashboard: http://%s\n", a.Cfg.Dashboard.Listen)
	}

	if err := config.WritePID(os.Getpid()); err != nil {
		return err
	}
	st := &config.RuntimeStatus{
		Mode:          "up",
		PID:           os.Getpid(),
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		TunnelID:      a.Creds.TunnelID,
		ProxyHostname: a.Cfg.ProxyHostnameOrDefault(),
		Dashboard:     "http://" + a.Cfg.Dashboard.Listen,
	}
	if includeProxy {
		st.ProxyListen = a.Cfg.Proxy.Listen
		st.ClientListen = a.Cfg.Proxy.ClientListen
	}
	_ = config.SaveStatus(st)
	defer config.ClearRuntimeFiles()

	printUpSummary(a, includeProxy)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	select {
	case <-ctx.Done():
	case <-sigc:
		fmt.Println("\nstopping...")
	}
	return nil
}

func printUpSummary(a *App, includeProxy bool) {
	fmt.Printf("tunnel %s (%s) is up\n", a.Cfg.TunnelName, a.Creds.TunnelID)
	if len(a.Cfg.Services) == 0 {
		fmt.Println("no public services yet; expose one with: cfpen expose 8080 --hostname app." + a.Cfg.ZoneName)
	}
	for _, s := range a.Cfg.Services {
		fmt.Printf("  %s -> %s  %s\n", s.Name, s.Service, config.PublicURL(s.Hostname))
	}
	if includeProxy {
		fmt.Printf("intranet proxy hostname: %s\n", a.Cfg.ProxyHostnameOrDefault())
		out, err := config.PathInDir(config.ClientFileName)
		if err == nil {
			_ = config.SaveClientBundle(out, a.ClientBundle())
			fmt.Printf("client bundle: %s\n", out)
			fmt.Printf("on the external machine: cfpen connect --token-file %s\n", filepath.Base(out))
		}
	}
}

func (a *App) dashboardStatus(running, includeProxy bool, proc *cloudflared.Process) dashboard.Status {
	st := dashboard.Status{
		Mode:      "up",
		Running:   running && (proc == nil || proc.Running()),
		TunnelID:  a.Creds.TunnelID,
		Dashboard: "http://" + a.Cfg.Dashboard.Listen,
	}
	if includeProxy {
		st.ProxyHostname = a.Cfg.ProxyHostnameOrDefault()
		st.ProxyListen = a.Cfg.Proxy.Listen
		st.ClientListen = a.Cfg.Proxy.ClientListen
	}
	return st
}

func (a *App) dashboardServices() []dashboard.Service {
	out := make([]dashboard.Service, 0, len(a.Cfg.Services))
	for _, s := range a.Cfg.Services {
		out = append(out, dashboard.Service{
			Name:     s.Name,
			Hostname: s.Hostname,
			Service:  s.Service,
			Path:     s.Path,
			URL:      config.PublicURL(s.Hostname),
		})
	}
	return out
}
