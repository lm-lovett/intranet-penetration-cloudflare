package cloudflared

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Process struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	logw   io.WriteCloser
	mu     sync.Mutex
}

type RunOptions struct {
	Bin     string
	Args    []string
	Env     []string
	LogFile string
	Dir     string
}

func Start(ctx context.Context, opt RunOptions) (*Process, error) {
	if opt.Bin == "" {
		return nil, fmt.Errorf("cloudflared binary path is empty")
	}
	cctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cctx, opt.Bin, opt.Args...)
	if opt.Dir != "" {
		cmd.Dir = opt.Dir
	}
	cmd.Env = append(os.Environ(), opt.Env...)

	var logw io.WriteCloser
	if opt.LogFile != "" {
		if err := os.MkdirAll(filepath.Dir(opt.LogFile), 0o700); err != nil {
			cancel()
			return nil, err
		}
		f, err := os.OpenFile(opt.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			cancel()
			return nil, err
		}
		logw = f
		cmd.Stdout = io.MultiWriter(os.Stdout, f)
		cmd.Stderr = io.MultiWriter(os.Stderr, f)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		cancel()
		if logw != nil {
			logw.Close()
		}
		return nil, err
	}
	p := &Process{cmd: cmd, cancel: cancel, logw: logw}
	go func() {
		_ = cmd.Wait()
	}()
	return p, nil
}

func TunnelArgs(metrics string, noAutoUpdate bool) []string {
	args := []string{"tunnel"}
	if noAutoUpdate {
		args = append(args, "--no-autoupdate")
	}
	if metrics != "" {
		args = append(args, "--metrics", metrics)
	}
	args = append(args, "run")
	return args
}

func AccessTCPArgs(hostname, listen, tokenID, tokenSecret string) []string {
	return []string{
		"access", "tcp",
		"--hostname", hostname,
		"--url", listen,
		"--service-token-id", tokenID,
		"--service-token-secret", tokenSecret,
	}
}

func (p *Process) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *Process) Running() bool {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	return p.cmd.ProcessState == nil
}

func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	if p.logw != nil {
		_ = p.logw.Close()
		p.logw = nil
	}
	return nil
}

func WaitReady(ctx context.Context, metrics string, timeout time.Duration) error {
	if metrics == "" {
		return nil
	}
	url := metricsURL(metrics)
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
			last = fmt.Errorf("metrics status %d", resp.StatusCode)
		} else {
			last = err
		}
		time.Sleep(400 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("timeout waiting for cloudflared")
	}
	return last
}

func metricsURL(metrics string) string {
	if strings.HasPrefix(metrics, "http://") || strings.HasPrefix(metrics, "https://") {
		return strings.TrimRight(metrics, "/") + "/ready"
	}
	return "http://" + metrics + "/ready"
}
