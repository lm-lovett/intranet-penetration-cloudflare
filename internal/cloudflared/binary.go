package cloudflared

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
)

const latestBase = "https://github.com/cloudflare/cloudflared/releases/latest/download"

func EnsureBinary(ctx context.Context, configured string) (string, error) {
	if configured != "" {
		if st, err := os.Stat(configured); err == nil && !st.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("cloudflared binary not found: %s", configured)
	}
	dir, err := config.BinDir()
	if err != nil {
		return "", err
	}
	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name = "cloudflared.exe"
	}
	dest := filepath.Join(dir, name)
	if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 0 {
		return dest, nil
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p, nil
	}
	asset, packed, err := releaseAsset()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	url := latestBase + "/" + asset
	if err := download(ctx, url, dest, packed); err != nil {
		return "", fmt.Errorf("download cloudflared: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dest, 0o755); err != nil {
			return "", err
		}
	}
	return dest, nil
}

func releaseAsset() (name string, packed bool, err error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "cloudflared-linux-amd64", false, nil
	case "linux/arm64":
		return "cloudflared-linux-arm64", false, nil
	case "linux/arm":
		return "cloudflared-linux-arm", false, nil
	case "darwin/amd64":
		return "cloudflared-darwin-amd64.tgz", true, nil
	case "darwin/arm64":
		return "cloudflared-darwin-arm64.tgz", true, nil
	case "windows/amd64":
		return "cloudflared-windows-amd64.exe", false, nil
	case "windows/arm64":
		return "cloudflared-windows-arm64.exe", false, nil
	default:
		return "", false, fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func download(ctx context.Context, url, dest string, packed bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "cfpen/0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d fetching %s", resp.StatusCode, url)
	}
	tmp := dest + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()
	if packed {
		if err := extractTGZBinary(resp.Body, f); err != nil {
			f.Close()
			return err
		}
	} else {
		if _, err := io.Copy(f, resp.Body); err != nil {
			f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func extractTGZBinary(r io.Reader, dest *os.File) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		base := filepath.Base(hdr.Name)
		if hdr.Typeflag == tar.TypeReg && (base == "cloudflared" || strings.HasPrefix(base, "cloudflared")) {
			_, err := io.Copy(dest, tr)
			return err
		}
	}
	return fmt.Errorf("cloudflared binary not found in archive")
}
