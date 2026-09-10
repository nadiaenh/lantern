package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Service is one monitored endpoint.
type Service struct {
	Name string
	URL  string
}

// Config is the parsed lantern.toml.
type Config struct {
	// Interval between check rounds. Default 15s.
	Interval time.Duration
	// Timeout for a single HTTP check. Default 5s.
	Timeout  time.Duration
	Services []Service
}

// loadConfig parses the small TOML subset lantern needs:
//
//	interval = "15s"
//	timeout  = "5s"
//	[[services]]
//	name = "podcast"
//	url  = "http://podcast-server:8080/health"
//
// ponytail: hand parser for this fixed shape; swap for BurntSushi/toml if the config grows.
func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &Config{Interval: 15 * time.Second, Timeout: 5 * time.Second}
	var cur *Service // non-nil while inside a [[services]] block
	flush := func() error {
		if cur == nil {
			return nil
		}
		if cur.Name == "" || cur.URL == "" {
			return fmt.Errorf("service needs both name and url: %+v", *cur)
		}
		cfg.Services = append(cfg.Services, *cur)
		cur = nil
		return nil
	}

	sc := bufio.NewScanner(f)
	for ln := 1; sc.Scan(); ln++ {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if line == "[[services]]" {
			if err := flush(); err != nil {
				return nil, err
			}
			cur = &Service{}
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value: %q", ln, line)
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch {
		case cur != nil && key == "name":
			cur.Name = val
		case cur != nil && key == "url":
			cur.URL = val
		case cur == nil && key == "interval":
			if cfg.Interval, err = time.ParseDuration(val); err != nil {
				return nil, fmt.Errorf("line %d: bad interval: %w", ln, err)
			}
		case cur == nil && key == "timeout":
			if cfg.Timeout, err = time.ParseDuration(val); err != nil {
				return nil, fmt.Errorf("line %d: bad timeout: %w", ln, err)
			}
		default:
			return nil, fmt.Errorf("line %d: unexpected key %q", ln, key)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("%s: no [[services]] defined", path)
	}
	return cfg, nil
}
