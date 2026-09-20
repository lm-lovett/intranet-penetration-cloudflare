package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/app"
	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/version"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "login":
		return cmdLogin(args[1:])
	case "up":
		return cmdUp(args[1:])
	case "down":
		return cmdDown(args[1:])
	case "expose":
		return cmdExpose(args[1:])
	case "unexpose", "rm":
		return cmdUnexpose(args[1:])
	case "ls", "list":
		return cmdList(args[1:])
	case "connect":
		return cmdConnect(args[1:])
	case "export-client":
		return cmdExport(args[1:])
	case "status":
		return cmdStatus(args[1:])
	case "version", "-v", "--version":
		fmt.Printf("cfpen %s (%s)\n", version.Version, version.Commit)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Print(`cfpen — Cloudflare dual-mode intranet tunnel

Usage:
  cfpen login --token <api_token> [--zone example.com]
  cfpen up
  cfpen expose <origin> [--hostname app.example.com] [--name web]
  cfpen connect --token-file client.json
  cfpen export-client [path]
  cfpen ls
  cfpen status
  cfpen down

Intranet (public HTTPS + SOCKS origin):
  cfpen login --token $CLOUDFLARE_API_TOKEN --zone example.com
  cfpen up
  cfpen expose 8080 --hostname app.example.com

External (all requests via intranet):
  cfpen connect --token-file client.json
  ALL_PROXY=socks5://127.0.0.1:1080 curl http://192.168.1.1
`)
}

func loadApp(fs *flag.FlagSet, args []string) (*app.App, []string, error) {
	var cfgPath, credsPath string
	fs.StringVar(&cfgPath, "config", "", "config yaml path")
	fs.StringVar(&credsPath, "creds", "", "credentials json path")
	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}
	a, err := app.Load(cfgPath, credsPath)
	return a, fs.Args(), err
}

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var token, account, zone, cfgPath, credsPath string
	fs.StringVar(&token, "token", "", "Cloudflare API token")
	fs.StringVar(&account, "account-id", "", "Cloudflare account id")
	fs.StringVar(&zone, "zone", "", "zone name, e.g. example.com")
	fs.StringVar(&cfgPath, "config", "", "config yaml path")
	fs.StringVar(&credsPath, "creds", "", "credentials json path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	a, err := app.Load(cfgPath, credsPath)
	if err != nil {
		return err
	}
	return a.Login(context.Background(), app.LoginOptions{
		Token:     token,
		AccountID: account,
		Zone:      zone,
	})
}

func cmdUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	noProxy := fs.Bool("no-proxy", false, "do not publish the intranet SOCKS proxy")
	noDash := fs.Bool("no-dashboard", false, "do not start the local dashboard")
	a, _, err := loadApp(fs, args)
	if err != nil {
		return err
	}
	return a.Up(context.Background(), app.UpOptions{NoProxy: *noProxy, NoDashboard: *noDash})
}

func cmdDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return app.Down()
}

func cmdExpose(args []string) error {
	fs := flag.NewFlagSet("expose", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var hostname, name, protocol, path string
	fs.StringVar(&hostname, "hostname", "", "public hostname")
	fs.StringVar(&name, "name", "", "service name")
	fs.StringVar(&protocol, "protocol", "http", "origin protocol: http, https, tcp, ssh, rdp")
	fs.StringVar(&path, "path", "", "optional path match")
	a, rest, err := loadApp(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: cfpen expose <origin> [--hostname host] [--name name]")
	}
	_, err = a.Expose(context.Background(), app.ExposeOptions{
		Origin:   rest[0],
		Hostname: hostname,
		Name:     name,
		Protocol: protocol,
		Path:     path,
	})
	return err
}

func cmdUnexpose(args []string) error {
	fs := flag.NewFlagSet("unexpose", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	a, rest, err := loadApp(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: cfpen unexpose <name>")
	}
	return a.Unexpose(context.Background(), rest[0])
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	a, _, err := loadApp(fs, args)
	if err != nil {
		return err
	}
	a.List()
	return nil
}

func cmdConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var tokenFile, listen string
	fs.StringVar(&tokenFile, "token-file", "", "client.json from cfpen export-client")
	fs.StringVar(&listen, "listen", "", "local proxy listen address")
	a, _, err := loadApp(fs, args)
	if err != nil {
		return err
	}
	return a.Connect(context.Background(), app.ConnectOptions{TokenFile: tokenFile, Listen: listen})
}

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export-client", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	a, rest, err := loadApp(fs, args)
	if err != nil {
		return err
	}
	path := ""
	if len(rest) > 0 {
		path = rest[0]
	}
	return a.ExportClient(path)
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	a, _, err := loadApp(fs, args)
	if err != nil {
		return err
	}
	return a.Status(context.Background())
}
