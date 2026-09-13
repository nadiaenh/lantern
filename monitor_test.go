package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestStatusTransitions(t *testing.T) {
	m := NewMonitor([]Service{{Name: "svc", URL: "http://x"}}, time.Second)
	now := time.Now()

	step := func(s Status, dt time.Duration) ServiceState {
		return m.apply("svc", Check{Time: now.Add(dt), Status: s})
	}

	// unknown -> up
	st := step(StatusUp, 0)
	if st.Status != StatusUp || !st.Since.Equal(now) || st.LastSuccess.IsZero() {
		t.Fatalf("first up: %+v", st)
	}

	// stays up
	st = step(StatusUp, time.Second)
	if !st.Since.Equal(now) {
		t.Fatalf("stable up moved Since: %+v", st)
	}

	// up -> down
	st = step(StatusDown, 2*time.Second)
	if st.Status != StatusDown || !st.Since.Equal(now.Add(2*time.Second)) || !st.LastSuccess.Equal(now.Add(time.Second)) {
		t.Fatalf("up->down: %+v", st)
	}

	// down -> up
	st = step(StatusUp, 3*time.Second)
	if st.Status != StatusUp || !st.LastSuccess.Equal(now.Add(3*time.Second)) {
		t.Fatalf("down->up: %+v", st)
	}
}

func TestHistoryCapped(t *testing.T) {
	m := NewMonitor([]Service{{Name: "svc", URL: "http://x"}}, time.Second)
	for i := 0; i < historyLen*2; i++ {
		m.apply("svc", Check{Time: time.Now(), Status: StatusUp})
	}
	if got := len(m.Snapshot()[0].History); got != historyLen {
		t.Fatalf("history len = %d, want %d", got, historyLen)
	}
}

func TestProbeTimeoutIsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 20 * time.Millisecond}
	c := probe(context.Background(), client, srv.URL)
	if c.Status != StatusDown || c.Err == "" {
		t.Fatalf("timeout should be down with error: %+v", c)
	}
}

func TestProbeClassifiesCodes(t *testing.T) {
	cases := map[int]Status{200: StatusUp, 301: StatusUp, 404: StatusDown, 503: StatusDown}
	for code, want := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		c := probe(context.Background(), srv.Client(), srv.URL)
		srv.Close()
		if c.Status != want {
			t.Errorf("code %d: got %s want %s", code, c.Status, want)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/lantern.toml"
	body := `
interval = "30s"
timeout = "3s"
# a comment
[[services]]
name = "podcast"
url = "http://podcast-server:8080/health"

[[services]]
name = "vm-broker"
url = "http://lab-server:8080/health"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval != 30*time.Second || cfg.Timeout != 3*time.Second {
		t.Fatalf("durations: %+v", cfg)
	}
	if len(cfg.Services) != 2 || cfg.Services[1].Name != "vm-broker" {
		t.Fatalf("services: %+v", cfg.Services)
	}
}
