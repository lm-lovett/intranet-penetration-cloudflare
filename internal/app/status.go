package app

import (
	"context"
	"fmt"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
)

func (a *App) Status(ctx context.Context) error {
	st, err := config.LoadStatus()
	if err != nil {
		return err
	}
	pid, _ := config.ReadPID()
	running := pidAlive(pid)
	if st == nil && !running {
		fmt.Println("cfpen is not running")
		a.List()
		return nil
	}
	mode := "unknown"
	if st != nil {
		mode = st.Mode
	}
	fmt.Printf("mode: %s\n", mode)
	if running {
		fmt.Printf("pid: %d (running)\n", pid)
	} else if pid > 0 {
		fmt.Printf("pid: %d (not running)\n", pid)
	}
	if st != nil {
		if st.TunnelID != "" {
			fmt.Printf("tunnel: %s\n", st.TunnelID)
		}
		if st.ProxyHostname != "" {
			fmt.Printf("proxy hostname: %s\n", st.ProxyHostname)
		}
		if st.ProxyListen != "" {
			fmt.Printf("intranet socks: %s\n", st.ProxyListen)
		}
		if st.ClientListen != "" {
			fmt.Printf("client listen: %s\n", st.ClientListen)
		}
		if st.Dashboard != "" {
			fmt.Printf("dashboard: %s\n", st.Dashboard)
		}
	}
	if a.Creds.TunnelID != "" {
		if cf, err := a.Client(); err == nil {
			if tun, err := cf.GetTunnel(ctx, a.Creds.TunnelID); err == nil {
				fmt.Printf("tunnel status: %s  connections: %d\n", tun.Status, len(tun.Connections))
			}
		}
	}
	a.List()
	return nil
}
