package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func testMonitor() *Monitor {
	m := NewMonitor([]Service{{Name: "podcast", URL: "http://x"}}, time.Second)
	m.apply("podcast", Check{Time: time.Now(), Status: StatusUp})
	return m
}

func TestServerAPI(t *testing.T) {
	srv := httptest.NewServer(newServer(testMonitor(), 15*time.Second))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var states []ServiceState
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].Name != "podcast" || states[0].Status != StatusUp {
		t.Fatalf("got %+v", states)
	}
}

func TestServerDashboard(t *testing.T) {
	srv := httptest.NewServer(newServer(testMonitor(), 15*time.Second))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	if !bytes.Contains(buf.Bytes(), []byte("podcast")) {
		t.Fatalf("dashboard missing service name: %s", buf.String())
	}
}
