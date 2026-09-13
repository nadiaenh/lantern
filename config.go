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
	Interval time.Duration
	Timeout  time.Duration
	Services []Service
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p := &configParser{cfg: &Config{Interval: 15 * time.Second, Timeout: 5 * time.Second}}
	sc := bufio.NewScanner(f)
	for ln := 1; sc.Scan(); ln++ {
		if err := p.feedLine(ln, sc.Text()); err != nil {
			return nil, err
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := p.flush(); err != nil {
		return nil, err
	}
	if len(p.cfg.Services) == 0 {
		return nil, fmt.Errorf("%s: no [[services]] defined", path)
	}
	return p.cfg, nil
}

type configParser struct {
	cfg *Config
	cur *Service
}

func (p *configParser) flush() error {
	if p.cur == nil {
		return nil
	}
	if p.cur.Name == "" || p.cur.URL == "" {
		return fmt.Errorf("service needs both name and url: %+v", *p.cur)
	}
	p.cfg.Services = append(p.cfg.Services, *p.cur)
	p.cur = nil
	return nil
}

func (p *configParser) feedLine(ln int, raw string) error {
	line := strings.TrimSpace(raw)
	if i := strings.Index(line, "#"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if line == "" {
		return nil
	}
	if line == "[[services]]" {
		if err := p.flush(); err != nil {
			return err
		}
		p.cur = &Service{}
		return nil
	}
	key, val, ok := strings.Cut(line, "=")
	if !ok {
		return fmt.Errorf("line %d: expected key = value: %q", ln, line)
	}
	return p.setField(ln, strings.TrimSpace(key), strings.Trim(strings.TrimSpace(val), `"`))
}

func (p *configParser) setField(ln int, key, val string) error {
	if p.cur != nil {
		switch key {
		case "name":
			p.cur.Name = val
		case "url":
			p.cur.URL = val
		default:
			return fmt.Errorf("line %d: unexpected key %q", ln, key)
		}
		return nil
	}
	var err error
	switch key {
	case "interval":
		if p.cfg.Interval, err = time.ParseDuration(val); err != nil {
			return fmt.Errorf("line %d: bad interval: %w", ln, err)
		}
	case "timeout":
		if p.cfg.Timeout, err = time.ParseDuration(val); err != nil {
			return fmt.Errorf("line %d: bad timeout: %w", ln, err)
		}
	default:
		return fmt.Errorf("line %d: unexpected key %q", ln, key)
	}
	return nil
}
