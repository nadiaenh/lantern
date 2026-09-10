package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Status is the reachability of a service.
type Status string

const (
	StatusUnknown Status = "unknown"
	StatusUp      Status = "up"
	StatusDown    Status = "down"
)

// Check is the result of one probe.
type Check struct {
	Time    time.Time     `json:"time"`
	Status  Status        `json:"status"`
	Latency time.Duration `json:"latency"`
	Err     string        `json:"error,omitempty"`
	Code    int           `json:"code,omitempty"`
}

// ServiceState is the live view of one service, kept in memory.
type ServiceState struct {
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Status      Status    `json:"status"`
	Since       time.Time `json:"since"`        // when Status last changed
	LastSuccess time.Time `json:"last_success"` // zero if never
	Last        Check     `json:"last"`
	History     []Check   `json:"history"` // newest last, capped
}

const historyLen = 50

// Monitor probes services on an interval and holds their state in memory.
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

// Run probes every service immediately, then every interval until ctx is done.
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

// probe performs one HTTP GET and classifies the outcome.
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

// apply folds a fresh check into a service's state and returns the updated copy.
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

// Snapshot returns a copy of every service state, in config order.
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
