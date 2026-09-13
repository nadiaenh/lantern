// Package main is lantern, a private uptime monitor for services on a tailnet.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(log.Ltime)
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "serve":
		serveCmd(os.Args[2:])
	case "diagnose":
		diagnoseCmd(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `lantern - private uptime monitor

usage:
  lantern serve    [--config lantern.toml] [--addr :8080]
  lantern diagnose <service> [--config lantern.toml]
`)
	os.Exit(2)
}

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := fs.String("config", "lantern.toml", "path to config file")
	addr := fs.String("addr", ":8080", "listen address for the dashboard")
	fs.Parse(args)

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	m := NewMonitor(cfg.Services, cfg.Timeout)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go m.Run(ctx, cfg.Interval)

	srv := &http.Server{Addr: *addr, Handler: newServer(m, cfg.Interval)}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	log.Printf("watching %d services every %s; dashboard on http://localhost%s", len(cfg.Services), cfg.Interval, *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func diagnoseCmd(args []string) {
	fs := flag.NewFlagSet("diagnose", flag.ExitOnError)
	cfgPath := fs.String("config", "lantern.toml", "path to config file")
	fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := diagnose(cfg, fs.Arg(0)); err != nil {
		log.Fatal(err)
	}
}
