package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Status is a service's reachability.
type Status string

const (
	StatusUnknown Status = "unknown"
	StatusUp      Status = "up"
	StatusDown    Status = "down"
)

// Check is the outcome of a single probe.
type Check struct {
	Time    time.Time     `json:"time"`
	Status  Status        `json:"status"`
	Latency time.Duration `json:"latency"`
	Err     string        `json:"error,omitempty"`
	Code    int           `json:"code,omitempty"`
}

// ServiceState is one service's current in-memory status.
type ServiceState struct {
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Status      Status    `json:"status"`
	Since       time.Time `json:"since"` // when Status last changed
	LastSuccess time.Time `json:"last_success"`
	Last        Check     `json:"last"`
	History     []Check   `json:"history"` // most recent last
}

const historyLen = 50

// Monitor periodically probes services and holds their latest state.
type Monitor struct {
	client  *http.Client
	timeout time.Duration

	mu     sync.RWMutex
	states map[string]*ServiceState
	order  []string
}

func NewMonitor(services []Service, timeout time.Duration) *Monitor {
	m := &Monitor{
		client:  &http.Client{Timeout: timeout},
		timeout: timeout,
		states:  make(map[string]*ServiceState, len(services)),
	}
	for _, s := range services {
		m.states[s.Name] = &ServiceState{Name: s.Name, URL: s.URL, Status: StatusUnknown}
		m.order = append(m.order, s.Name)
	}
	return m
}

// Run probes immediately, then on every tick until ctx is canceled.
func (m *Monitor) Run(ctx context.Context, interval time.Duration) {
	m.probeAll(ctx)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.probeAll(ctx)
		}
	}
}

func (m *Monitor) probeAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, name := range m.order {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			m.mu.RLock()
			url := m.states[name].URL
			m.mu.RUnlock()
			m.apply(name, probe(ctx, m.client, url))
		}(name)
	}
	wg.Wait()
}

// probe issues one HTTP GET and classifies the result as up or down.
func probe(ctx context.Context, client *http.Client, url string) Check {
	start := time.Now()
	c := Check{Time: start}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.Status, c.Err = StatusDown, err.Error()
		return c
	}
	resp, err := client.Do(req)
	c.Latency = time.Since(start)
	if err != nil {
		c.Status, c.Err = StatusDown, err.Error()
		return c
	}
	resp.Body.Close()
	c.Code = resp.StatusCode
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		c.Status = StatusUp
	} else {
		c.Status, c.Err = StatusDown, resp.Status
	}
	return c
}

// apply records a check into the named service's state.
func (m *Monitor) apply(name string, c Check) ServiceState {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.states[name]
	if st.Status != c.Status {
		st.Status = c.Status
		st.Since = c.Time
	}
	if c.Status == StatusUp {
		st.LastSuccess = c.Time
	}
	st.Last = c
	st.History = append(st.History, c)
	if len(st.History) > historyLen {
		st.History = st.History[len(st.History)-historyLen:]
	}
	return *st
}

// Snapshot returns every service's current state, in config order.
func (m *Monitor) Snapshot() []ServiceState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServiceState, 0, len(m.order))
	for _, name := range m.order {
		st := *m.states[name]
		st.History = append([]Check(nil), st.History...)
		out = append(out, st)
	}
	return out
}
