package app

import (
	"fmt"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/cloudflare"
	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
)

type App struct {
	ConfigPath string
	CredsPath  string
	Cfg        *config.Config
	Creds      *config.Credentials
}

func Load(configPath, credsPath string) (*App, error) {
	cfg, creds, cfgFile, err := config.LoadResolved(configPath, credsPath)
	if err != nil {
		return nil, err
	}
	if credsPath == "" {
		_, credsPath, err = config.DefaultPaths()
		if err != nil {
			return nil, err
		}
	}
	if cfgFile == "" {
		cfgFile, _, err = config.DefaultPaths()
		if err != nil {
			return nil, err
		}
	}
	return &App{ConfigPath: cfgFile, CredsPath: credsPath, Cfg: cfg, Creds: creds}, nil
}

func (a *App) Reload() error {
	cfg, creds, cfgFile, err := config.LoadResolved(a.ConfigPath, a.CredsPath)
	if err != nil {
		return err
	}
	a.Cfg = cfg
	a.Creds = creds
	if cfgFile != "" {
		a.ConfigPath = cfgFile
	}
	return nil
}

func (a *App) Save() error {
	if err := config.Save(a.ConfigPath, a.Cfg); err != nil {
		return err
	}
	return config.SaveCredentials(a.CredsPath, a.Creds)
}

func (a *App) Client() (*cloudflare.Client, error) {
	token := a.Cfg.APIToken
	if token == "" {
		token = a.Creds.APIToken
	}
	if token == "" {
		return nil, fmt.Errorf("missing API token; run: cfpen login --token <token>")
	}
	accountID := a.Cfg.AccountID
	if accountID == "" {
		accountID = a.Creds.AccountID
	}
	return cloudflare.New(token, accountID), nil
}

func (a *App) RequireZone() error {
	if a.Cfg.ZoneID == "" || a.Cfg.ZoneName == "" {
		return fmt.Errorf("no Cloudflare zone configured; run: cfpen login --zone example.com")
	}
	return nil
}

func ingressRules(cfg *config.Config, includeProxy bool) []cloudflare.IngressRule {
	hosts := make([]string, 0, len(cfg.Services))
	origins := make([]string, 0, len(cfg.Services))
	paths := make([]string, 0, len(cfg.Services))
	for _, s := range cfg.Services {
		hosts = append(hosts, s.Hostname)
		origins = append(origins, s.Service)
		paths = append(paths, s.Path)
	}
	proxyHost, proxyOrigin := "", ""
	if includeProxy && !cfg.Proxy.Disable {
		proxyHost = cfg.ProxyHostnameOrDefault()
		proxyOrigin = cfg.ProxyOrigin()
	}
	return cloudflare.IngressFromConfig(hosts, origins, paths, proxyHost, proxyOrigin)
}
