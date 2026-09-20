package cloudflare

import (
	"fmt"
	"time"
)

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return "cloudflare api error"
	}
	return fmt.Sprintf("cloudflare api: %s (status %d)", e.Message, e.Status)
}

func IsNotFound(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Status == 404
	}
	return false
}

type Envelope[T any] struct {
	Success  bool       `json:"success"`
	Errors   []APIError `json:"errors"`
	Messages []APIError `json:"messages"`
	Result   T          `json:"result"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type TokenVerify struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Tunnel struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	AccountTag  string       `json:"account_tag"`
	CreatedAt   time.Time    `json:"created_at"`
	DeletedAt   *time.Time   `json:"deleted_at"`
	Status      string       `json:"status"`
	TunType     string       `json:"tun_type"`
	ConfigSrc   string       `json:"config_src"`
	Token       string       `json:"token"`
	Connections []Connection `json:"connections"`
}

type Connection struct {
	ID                 string    `json:"id"`
	ConnectedAt        time.Time `json:"connected_at"`
	OriginIP           string    `json:"origin_ip"`
	OpenedAt           time.Time `json:"opened_at"`
	ClientID           string    `json:"client_id"`
	ClientVersion      string    `json:"client_version"`
	IsPendingReconnect bool      `json:"is_pending_reconnect"`
}

type TunnelConfig struct {
	Config IngressConfig `json:"config"`
}

type IngressConfig struct {
	Ingress []IngressRule `json:"ingress"`
}

type IngressRule struct {
	Hostname      string         `json:"hostname,omitempty"`
	Path          string         `json:"path,omitempty"`
	Service       string         `json:"service"`
	OriginRequest map[string]any `json:"originRequest,omitempty"`
}

type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
	Comment string `json:"comment,omitempty"`
}

type AccessOrganization struct {
	Name       string `json:"name"`
	AuthDomain string `json:"auth_domain"`
}

type AccessApp struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Type   string `json:"type"`
}

type AccessPolicy struct {
	ID       string           `json:"id,omitempty"`
	Name     string           `json:"name"`
	Decision string           `json:"decision"`
	Include  []map[string]any `json:"include"`
}

type ServiceToken struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
}
