package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// diagnose isolates DNS, Tailscale, and HTTP so each failure mode is distinguishable.
func diagnose(cfg *Config, name string) error {
	var svc *Service
	for i := range cfg.Services {
		if cfg.Services[i].Name == name {
			svc = &cfg.Services[i]
		}
	}
	if svc == nil {
		return fmt.Errorf("no service named %q in config", name)
	}
	fmt.Printf("diagnosing %s (%s)\n\n", svc.Name, svc.URL)

	u, err := url.Parse(svc.URL)
	if err != nil {
		return fmt.Errorf("bad url: %w", err)
	}
	host := u.Hostname()

	// DNS resolution.
	ips, err := net.LookupHost(host)
	if err != nil {
		fmt.Printf("DNS        FAIL  %v\n", err)
	} else {
		fmt.Printf("DNS        ok    %s -> %s\n", host, strings.Join(ips, ", "))
	}

	// Tailscale reachability.
	if _, err := exec.LookPath("tailscale"); err != nil {
		fmt.Printf("Tailscale  skip  tailscale CLI not found\n")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		out, err := exec.CommandContext(ctx, "tailscale", "ping", "-c", "1", host).CombinedOutput()
		cancel()
		line := firstLine(out)
		if err != nil {
			fmt.Printf("Tailscale  FAIL  %s\n", line)
		} else {
			fmt.Printf("Tailscale  ok    %s\n", line)
		}
	}

	// HTTP health check.
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, svc.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("HTTP       FAIL  %v (after %s)\n", err, time.Since(start).Round(time.Millisecond))
		return nil
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	verdict := "ok"
	if resp.StatusCode >= 400 {
		verdict = "FAIL"
	}
	fmt.Printf("HTTP       %s    %s in %s\n", verdict, resp.Status, time.Since(start).Round(time.Millisecond))
	return nil
}

// firstLine returns the first non-warning line of tailscale's output.
func firstLine(b []byte) string {
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "Warning:") {
			continue
		}
		return ln
	}
	return strings.TrimSpace(string(b))
}
