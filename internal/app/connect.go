package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/cloudflared"
	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
)

type ConnectOptions struct {
	TokenFile string
	Listen    string
}

func (a *App) Connect(ctx context.Context, opt ConnectOptions) error {
	bundle, err := loadBundle(a, opt.TokenFile)
	if err != nil {
		return err
	}
	listen := opt.Listen
	if listen == "" {
		listen = bundle.Listen
	}
	if listen == "" {
		listen = a.Cfg.Proxy.ClientListen
	}
	if listen == "" {
		listen = config.DefaultClientListen
	}
	bin, err := cloudflared.EnsureBinary(ctx, a.Cfg.Cloudflared.Bin)
	if err != nil {
		return err
	}
	logFile, err := config.PathInDir("access.log")
	if err != nil {
		return err
	}
	proc, err := cloudflared.Start(ctx, cloudflared.RunOptions{
		Bin:     bin,
		Args:    cloudflared.AccessTCPArgs(bundle.Hostname, listen, bundle.ClientID, bundle.ClientSecret),
		LogFile: logFile,
	})
	if err != nil {
		return fmt.Errorf("start cloudflared access: %w", err)
	}
	defer proc.Stop()

	_ = config.WritePID(os.Getpid())
	_ = config.SaveStatus(&config.RuntimeStatus{
		Mode:          "connect",
		PID:           os.Getpid(),
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		ProxyHostname: bundle.Hostname,
		ClientListen:  listen,
	})
	defer config.ClearRuntimeFiles()

	fmt.Printf("connected via %s\n", bundle.Hostname)
	fmt.Printf("local SOCKS5/HTTP proxy: %s\n", listen)
	fmt.Printf("example: curl --proxy socks5://%s http://127.0.0.1:80\n", listen)
	fmt.Printf("example: ALL_PROXY=socks5://%s curl http://192.168.1.1\n", listen)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	select {
	case <-ctx.Done():
	case <-sigc:
		fmt.Println("\nstopping...")
	}
	return nil
}

func (a *App) ExportClient(path string) error {
	if path == "" {
		var err error
		path, err = config.PathInDir(config.ClientFileName)
		if err != nil {
			return err
		}
	}
	b := a.ClientBundle()
	if b.Hostname == "" || b.ClientID == "" || b.ClientSecret == "" {
		return fmt.Errorf("client credentials not ready; run cfpen up on the intranet host first")
	}
	if err := config.SaveClientBundle(path, b); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", path)
	fmt.Println("copy this file to the external machine (it does not contain your API token)")
	return nil
}

func loadBundle(a *App, tokenFile string) (*config.ClientBundle, error) {
	candidates := []string{}
	if tokenFile != "" {
		candidates = append(candidates, tokenFile)
	}
	if p, err := config.PathInDir(config.ClientFileName); err == nil {
		candidates = append(candidates, p)
	}
	if a.CredsPath != "" {
		candidates = append(candidates, a.CredsPath)
	}
	var last error
	for _, p := range candidates {
		b, err := config.LoadClientBundle(p)
		if err == nil {
			return b, nil
		}
		last = err
	}
	if a.Creds.AccessClientID != "" && a.Creds.AccessSecret != "" {
		return a.ClientBundle(), nil
	}
	if last != nil {
		return nil, fmt.Errorf("load client bundle: %w", last)
	}
	return nil, fmt.Errorf("missing client bundle; copy client.json and pass --token-file")
}
