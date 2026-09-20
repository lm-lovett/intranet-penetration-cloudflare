package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DirName             = ".cfpen"
	ConfigFileName      = "config.yaml"
	CredsFileName       = "credentials.json"
	ClientFileName      = "client.json"
	PidFileName         = "cfpen.pid"
	StatusFileName      = "status.json"
	DefaultListen       = "127.0.0.1:4090"
	DefaultTunnel       = "cfpen"
	DefaultLocalHost    = "127.0.0.1"
	DefaultProxyListen  = "127.0.0.1:41080"
	DefaultClientListen = "127.0.0.1:1080"
	DefaultProxySub     = "proxy"
	DefaultMetrics      = "127.0.0.1:4099"
)

type Config struct {
	AccountID   string      `yaml:"account_id" json:"account_id"`
	APIToken    string      `yaml:"api_token" json:"api_token"`
	ZoneID      string      `yaml:"zone_id" json:"zone_id"`
	ZoneName    string      `yaml:"zone_name" json:"zone_name"`
	TunnelName  string      `yaml:"tunnel_name" json:"tunnel_name"`
	Dashboard   Dashboard   `yaml:"dashboard" json:"dashboard"`
	Cloudflared Cloudflared `yaml:"cloudflared" json:"cloudflared"`
	Worker      Worker      `yaml:"worker" json:"worker"`
	Proxy       Proxy       `yaml:"proxy" json:"proxy"`
	Services    []Service   `yaml:"services" json:"services"`
}

type Service struct {
	Name     string `yaml:"name" json:"name"`
	Hostname string `yaml:"hostname" json:"hostname"`
	Service  string `yaml:"service" json:"service"`
	Path     string `yaml:"path" json:"path"`
}

type Dashboard struct {
	Listen  string `yaml:"listen" json:"listen"`
	Disable bool   `yaml:"disable" json:"disable"`
}

type Cloudflared struct {
	Bin          string `yaml:"bin" json:"bin"`
	Metrics      string `yaml:"metrics" json:"metrics"`
	NoAutoUpdate bool   `yaml:"no_autoupdate" json:"no_autoupdate"`
}

type Worker struct {
	URL   string `yaml:"url" json:"url"`
	Token string `yaml:"token" json:"token"`
}

type Proxy struct {
	Hostname     string `yaml:"hostname" json:"hostname"`
	Listen       string `yaml:"listen" json:"listen"`
	ClientListen string `yaml:"client_listen" json:"client_listen"`
	Disable      bool   `yaml:"disable" json:"disable"`
}

type Credentials struct {
	APIToken       string `json:"api_token,omitempty"`
	AccountID      string `json:"account_id,omitempty"`
	ZoneID         string `json:"zone_id,omitempty"`
	ZoneName       string `json:"zone_name,omitempty"`
	TunnelID       string `json:"tunnel_id,omitempty"`
	TunnelToken    string `json:"tunnel_token,omitempty"`
	AccessAppID    string `json:"access_app_id,omitempty"`
	AccessClientID string `json:"access_client_id,omitempty"`
	AccessSecret   string `json:"access_client_secret,omitempty"`
	ProxyHostname  string `json:"proxy_hostname,omitempty"`
}

type ClientBundle struct {
	Hostname     string `json:"hostname"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Listen       string `json:"listen,omitempty"`
}

type RuntimeStatus struct {
	Mode          string `json:"mode"`
	PID           int    `json:"pid"`
	StartedAt     string `json:"started_at"`
	TunnelID      string `json:"tunnel_id,omitempty"`
	ProxyListen   string `json:"proxy_listen,omitempty"`
	ProxyHostname string `json:"proxy_hostname,omitempty"`
	Dashboard     string `json:"dashboard,omitempty"`
	ClientListen  string `json:"client_listen,omitempty"`
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DirName), nil
}

func DefaultPaths() (configPath, credsPath string, err error) {
	dir, err := Dir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, ConfigFileName), filepath.Join(dir, CredsFileName), nil
}

func PathInDir(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func PidPath() (string, error) {
	return PathInDir(PidFileName)
}

func StatusFilePath() (string, error) {
	return PathInDir(StatusFileName)
}

func BinDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin"), nil
}

func SearchConfig(explicit string) string {
	if explicit != "" {
		return explicit
	}
	candidates := []string{
		"cfpen.yaml",
		"cfpen.yml",
	}
	if dir, err := Dir(); err == nil {
		candidates = append(candidates, filepath.Join(dir, ConfigFileName))
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func Load(path string) (*Config, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.ApplyDefaults()
	return cfg, nil
}

func Defaults() *Config {
	cfg := &Config{}
	cfg.ApplyDefaults()
	return cfg
}

func (c *Config) ApplyDefaults() {
	if c.TunnelName == "" {
		c.TunnelName = DefaultTunnel
	}
	if c.Dashboard.Listen == "" {
		c.Dashboard.Listen = DefaultListen
	}
	if c.Cloudflared.Metrics == "" {
		c.Cloudflared.Metrics = DefaultMetrics
	}
	if !c.Cloudflared.NoAutoUpdate {
		c.Cloudflared.NoAutoUpdate = true
	}
	if c.Proxy.Listen == "" {
		c.Proxy.Listen = DefaultProxyListen
	}
	if c.Proxy.ClientListen == "" {
		c.Proxy.ClientListen = DefaultClientListen
	}
}

func (c *Config) MergeCredentials(creds *Credentials) {
	if creds == nil {
		return
	}
	if c.APIToken == "" {
		c.APIToken = creds.APIToken
	}
	if c.AccountID == "" {
		c.AccountID = creds.AccountID
	}
	if c.ZoneID == "" {
		c.ZoneID = creds.ZoneID
	}
	if c.ZoneName == "" {
		c.ZoneName = creds.ZoneName
	}
	if c.Proxy.Hostname == "" {
		c.Proxy.Hostname = creds.ProxyHostname
	}
}

func (c *Config) MergeEnv() {
	if v := os.Getenv("CLOUDFLARE_API_TOKEN"); v != "" && c.APIToken == "" {
		c.APIToken = v
	}
	if v := os.Getenv("CLOUDFLARE_ACCOUNT_ID"); v != "" && c.AccountID == "" {
		c.AccountID = v
	}
	if v := os.Getenv("CLOUDFLARE_ZONE_ID"); v != "" && c.ZoneID == "" {
		c.ZoneID = v
	}
	if v := os.Getenv("CFP_WORKER_URL"); v != "" && c.Worker.URL == "" {
		c.Worker.URL = v
	}
	if v := os.Getenv("CFP_WORKER_TOKEN"); v != "" && c.Worker.Token == "" {
		c.Worker.Token = v
	}
}

func (c *Config) ProxyHostnameOrDefault() string {
	if c.Proxy.Hostname != "" {
		return c.Proxy.Hostname
	}
	if c.ZoneName != "" {
		return DefaultProxySub + "." + c.ZoneName
	}
	return ""
}

func (c *Config) ProxyOrigin() string {
	listen := strings.TrimSpace(c.Proxy.Listen)
	if listen == "" {
		listen = DefaultProxyListen
	}
	if strings.Contains(listen, "://") {
		return listen
	}
	return "tcp://" + listen
}

func (c *Config) FindService(name string) (int, Service, bool) {
	for i, s := range c.Services {
		if s.Name == name || s.Hostname == name {
			return i, s, true
		}
	}
	return -1, Service{}, false
}

func (c *Config) UpsertService(svc Service) {
	if i, _, ok := c.FindService(svc.Name); ok {
		c.Services[i] = svc
		return
	}
	c.Services = append(c.Services, svc)
}

func (c *Config) RemoveService(name string) bool {
	i, _, ok := c.FindService(name)
	if !ok {
		return false
	}
	c.Services = append(c.Services[:i], c.Services[i+1:]...)
	return true
}

func LoadCredentials(path string) (*Credentials, error) {
	if path == "" {
		_, p, err := DefaultPaths()
		if err != nil {
			return nil, err
		}
		path = p
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Credentials{}, nil
		}
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(raw, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return &creds, nil
}

func SaveCredentials(path string, creds *Credentials) error {
	if path == "" {
		_, p, err := DefaultPaths()
		if err != nil {
			return err
		}
		path = p
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func Save(path string, cfg *Config) error {
	if path == "" {
		p, _, err := DefaultPaths()
		if err != nil {
			return err
		}
		path = p
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func LoadResolved(configPath, credsPath string) (*Config, *Credentials, string, error) {
	cfgFile := SearchConfig(configPath)
	cfg, err := Load(cfgFile)
	if err != nil {
		return nil, nil, "", err
	}
	if credsPath == "" {
		_, credsPath, err = DefaultPaths()
		if err != nil {
			return nil, nil, "", err
		}
	}
	creds, err := LoadCredentials(credsPath)
	if err != nil {
		return nil, nil, "", err
	}
	cfg.MergeCredentials(creds)
	cfg.MergeEnv()
	cfg.ApplyDefaults()
	return cfg, creds, cfgFile, nil
}

func (c *Credentials) ClientBundle(hostname, listen string) *ClientBundle {
	if hostname == "" {
		hostname = c.ProxyHostname
	}
	return &ClientBundle{
		Hostname:     hostname,
		ClientID:     c.AccessClientID,
		ClientSecret: c.AccessSecret,
		Listen:       listen,
	}
}

func LoadClientBundle(path string) (*ClientBundle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bundle ClientBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return nil, fmt.Errorf("parse client bundle: %w", err)
	}
	if bundle.Hostname == "" || bundle.ClientID == "" {
		var creds Credentials
		if err := json.Unmarshal(raw, &creds); err == nil && creds.ProxyHostname != "" && creds.AccessClientID != "" {
			return creds.ClientBundle(creds.ProxyHostname, bundle.Listen), nil
		}
		return nil, errors.New("client bundle missing hostname or client_id")
	}
	if bundle.ClientSecret == "" {
		return nil, errors.New("client bundle missing client_secret")
	}
	return &bundle, nil
}

func SaveClientBundle(path string, bundle *ClientBundle) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func SaveStatus(st *RuntimeStatus) error {
	path, err := StatusFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func LoadStatus() (*RuntimeStatus, error) {
	path, err := StatusFilePath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var st RuntimeStatus
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func WritePID(pid int) error {
	path, err := PidPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", pid)), 0o600)
}

func ReadPID() (int, error) {
	path, err := PidPath()
	if err != nil {
		return 0, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var pid int
	_, err = fmt.Sscanf(string(raw), "%d", &pid)
	return pid, err
}

func ClearRuntimeFiles() {
	if p, err := PidPath(); err == nil {
		_ = os.Remove(p)
	}
	if p, err := StatusFilePath(); err == nil {
		_ = os.Remove(p)
	}
}

// ParseOrigin turns user input like "8080", "localhost:3000", or a full URL
// into a Cloudflare Tunnel origin service string.
func ParseOrigin(raw, protocol string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty origin")
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "unix:") || strings.HasPrefix(raw, "http_status:") {
		return raw, nil
	}
	host := DefaultLocalHost
	port := raw
	if strings.Contains(raw, ":") && !strings.HasPrefix(raw, ":") {
		host, port, _ = strings.Cut(raw, ":")
		if host == "" {
			host = DefaultLocalHost
		}
	} else {
		port = strings.TrimPrefix(raw, ":")
	}
	if port == "" {
		return "", fmt.Errorf("invalid origin %q", raw)
	}
	scheme := "http"
	switch protocol {
	case "", "http":
		scheme = "http"
	case "https", "http2":
		scheme = protocol
	case "ssh":
		scheme = "ssh"
	case "rdp":
		scheme = "rdp"
	case "tcp", "smb":
		scheme = "tcp"
	case "unix":
		return "unix:" + raw, nil
	default:
		return "", fmt.Errorf("unsupported protocol %q", protocol)
	}
	return fmt.Sprintf("%s://%s:%s", scheme, host, port), nil
}

func PublicURL(hostname string) string {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return ""
	}
	if strings.HasPrefix(hostname, "http://") || strings.HasPrefix(hostname, "https://") {
		return hostname
	}
	return "https://" + hostname
}

func HostnameZone(hostname string) string {
	hostname = strings.TrimSpace(hostname)
	hostname = strings.TrimPrefix(hostname, "https://")
	hostname = strings.TrimPrefix(hostname, "http://")
	if i := strings.IndexByte(hostname, '/'); i >= 0 {
		hostname = hostname[:i]
	}
	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return hostname
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func SuggestServiceName(hostname, origin string) string {
	if hostname != "" {
		host := hostname
		host = strings.TrimPrefix(host, "https://")
		host = strings.TrimPrefix(host, "http://")
		if i := strings.IndexByte(host, '.'); i > 0 {
			return host[:i]
		}
		if host != "" {
			return host
		}
	}
	if origin != "" {
		if i := strings.LastIndex(origin, ":"); i >= 0 && i < len(origin)-1 {
			return "port-" + origin[i+1:]
		}
	}
	return "app"
}
