package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestDashboardAPI(t *testing.T) {
	s := New("127.0.0.1:0", Hooks{
		Status: func() Status {
			return Status{Mode: "up", Running: true, TunnelID: "tun"}
		},
		List: func() []Service {
			return []Service{{Name: "web", Hostname: "web.example.com", URL: "https://web.example.com"}}
		},
		Expose: func(ExposeRequest) error { return nil },
		Remove: func(string) error { return nil },
		Client: func() (any, error) {
			return map[string]string{"hostname": "proxy.example.com"}, nil
		},
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := "http://" + s.ln.Addr().String()
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	var err error
	for time.Now().Before(deadline) {
		resp, err = http.Get(base + "/api/status")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var st Status
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if !st.Running || st.TunnelID != "tun" {
		t.Fatalf("%+v", st)
	}
	html, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(html.Body)
	html.Body.Close()
	if html.StatusCode != 200 || len(b) < 100 {
		t.Fatalf("index status %d len %d", html.StatusCode, len(b))
	}
}
