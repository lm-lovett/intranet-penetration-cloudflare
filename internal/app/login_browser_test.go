package app

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestBrowserCommandLinuxOrDarwin(t *testing.T) {
	name, args := browserCommand("https://example.com")
	if name == "" || len(args) == 0 || args[len(args)-1] != "https://example.com" {
		t.Fatalf("cmd %s %v", name, args)
	}
}

func TestCollectTokenViaBrowserHTTP(t *testing.T) {
	opened := make(chan string, 1)
	done := make(chan struct{})
	var got string
	var err error
	go func() {
		got, err = CollectTokenViaBrowser(context.Background(), CollectTokenOptions{
			Open: func(u string) error {
				opened <- u
				return nil
			},
			Stdin:   strings.NewReader(""),
			Stdout:  io.Discard,
			Timeout: 5 * time.Second,
		})
		close(done)
	}()

	var local string
	select {
	case local = <-opened:
	case <-time.After(3 * time.Second):
		t.Fatal("browser was not opened")
	}

	var body []byte
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, getErr := http.Get(local)
		if getErr == nil {
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 && len(body) > 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	html := string(body)
	if !strings.Contains(html, "打开 Cloudflare 登录") {
		t.Fatalf("login page: %s", html)
	}
	state := findAttr(html, "name=\"state\"")
	if state == "" {
		re := regexp.MustCompile(`name="state" value="([^"]+)"`)
		m := re.FindStringSubmatch(html)
		if len(m) != 2 {
			t.Fatalf("no state in %s", html)
		}
		state = m[1]
	}
	postURL := strings.TrimSuffix(local, "/") + "/done"
	resp, postErr := http.PostForm(postURL, url.Values{
		"state": {state},
		"token": {"browser-token-value"},
	})
	if postErr != nil {
		t.Fatal(postErr)
	}
	resp.Body.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("collect did not finish")
	}
	if err != nil {
		t.Fatal(err)
	}
	if got != "browser-token-value" {
		t.Fatalf("got %q", got)
	}
}

func TestCollectTokenViaStdin(t *testing.T) {
	got, err := CollectTokenViaBrowser(context.Background(), CollectTokenOptions{
		NoBrowser: true,
		Open:      func(string) error { return nil },
		Stdin:     strings.NewReader("stdin-token-value\n"),
		Stdout:    io.Discard,
		Timeout:   3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "stdin-token-value" {
		t.Fatalf("got %q", got)
	}
}

func findAttr(html, needle string) string {
	re := regexp.MustCompile(needle + ` value="([^"]+)"`)
	m := re.FindStringSubmatch(html)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}
