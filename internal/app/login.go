package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/cloudflare"
)

type LoginOptions struct {
	Token     string
	AccountID string
	Zone      string
	NoBrowser bool
}

func (a *App) Login(ctx context.Context, opt LoginOptions) error {
	token := strings.TrimSpace(opt.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	}
	if token == "" {
		tok, err := CollectTokenViaBrowser(ctx, CollectTokenOptions{NoBrowser: opt.NoBrowser})
		if err != nil {
			return err
		}
		token = tok
	}
	return a.finishLogin(ctx, token, opt)
}

func (a *App) finishLogin(ctx context.Context, token string, opt LoginOptions) error {
	cf := cloudflare.New(token, opt.AccountID)
	info, err := cf.VerifyToken(ctx)
	if err != nil {
		return fmt.Errorf("verify token: %w", err)
	}
	if info.Status != "" && !strings.EqualFold(info.Status, "active") {
		return fmt.Errorf("token status is %s, expected active", info.Status)
	}
	accountID := opt.AccountID
	if accountID == "" {
		id, err := cf.EnsureAccountID(ctx)
		if err != nil {
			return err
		}
		accountID = id
	} else {
		cf.AccountID = accountID
	}
	accounts, err := cf.ListAccounts(ctx)
	if err != nil {
		return err
	}
	accountName := accountID
	for _, acc := range accounts {
		if acc.ID == accountID {
			accountName = acc.Name
			break
		}
	}

	var zone *cloudflare.Zone
	if opt.Zone != "" {
		zones, err := cf.ListZones(ctx, opt.Zone)
		if err != nil {
			return err
		}
		for i := range zones {
			if strings.EqualFold(zones[i].Name, opt.Zone) || zones[i].ID == opt.Zone {
				zone = &zones[i]
				break
			}
		}
		if zone == nil {
			return fmt.Errorf("zone %q not found", opt.Zone)
		}
	} else {
		zones, err := cf.ListZones(ctx, "")
		if err != nil {
			return err
		}
		if len(zones) == 0 {
			return fmt.Errorf("no zones on this account; add a domain to Cloudflare first")
		}
		zone = &zones[0]
		if len(zones) > 1 {
			fmt.Fprintf(os.Stderr, "note: multiple zones found, using %s (pass --zone to pick another)\n", zone.Name)
		}
	}

	a.Cfg.APIToken = ""
	a.Cfg.AccountID = accountID
	a.Cfg.ZoneID = zone.ID
	a.Cfg.ZoneName = zone.Name
	a.Cfg.ApplyDefaults()
	a.Creds.APIToken = token
	a.Creds.AccountID = accountID
	a.Creds.ZoneID = zone.ID
	a.Creds.ZoneName = zone.Name
	if err := a.Save(); err != nil {
		return err
	}
	fmt.Printf("logged in as account %s (%s), zone %s\n", accountName, accountID, zone.Name)
	fmt.Printf("credentials: %s\n", a.CredsPath)
	fmt.Printf("config: %s\n", a.ConfigPath)
	return nil
}
