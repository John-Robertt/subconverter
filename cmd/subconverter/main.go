// Command subconverter runs the proxy configuration generation HTTP service.
//
// Usage:
//
//	subconverter -config <path> [-listen :8080] [-cache-ttl 5m] [-timeout 30s]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/server"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "", "path to YAML config file (required)")
	listen := flag.String("listen", ":8080", "listen address")
	cacheTTL := flag.Duration("cache-ttl", 5*time.Minute, "subscription and template cache TTL")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP fetch timeout for subscriptions")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("subconverter %s\ncommit: %s\nbuilt: %s\n", version, commit, date)
		return
	}

	if *configPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("starting subconverter %s (commit=%s built=%s)", version, commit, date)

	// Build dependency chain: HTTPFetcher -> CachedFetcher.
	httpFetcher := &fetch.HTTPFetcher{Client: &http.Client{Timeout: *timeout}}
	cachedFetcher := fetch.NewCachedFetcher(httpFetcher, *cacheTTL)

	// Load and validate config at startup (fail-fast).
	cfg, err := config.Load(context.Background(), *configPath, cachedFetcher)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if err := config.Validate(cfg); err != nil {
		log.Fatalf("config validation failed:\n%v", err)
	}

	// Start HTTP server.
	srv := server.New(cfg, cachedFetcher)
	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on %s", *listen)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
