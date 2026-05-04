// Command subconverter runs the proxy configuration generation HTTP service.
//
// Usage:
//
//	subconverter -config <path> [-listen :8080] [-cache-ttl 5m] [-timeout 30s] [-access-token secret]
//	subconverter -healthcheck [-listen :8080]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/John-Robertt/subconverter/internal/admin"
	"github.com/John-Robertt/subconverter/internal/app"
	"github.com/John-Robertt/subconverter/internal/auth"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/generate"
	"github.com/John-Robertt/subconverter/internal/server"
	"github.com/John-Robertt/subconverter/internal/webui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	defaultListenAddr = ":8080"
	listenEnvVar      = "SUBCONVERTER_LISTEN"
	//nolint:gosec // This is an environment variable name, not an embedded credential.
	accessTokenEnvVar = "SUBCONVERTER_TOKEN"
	authStateEnvVar   = "SUBCONVERTER_AUTH_STATE"
	setupTokenEnvVar  = "SUBCONVERTER_SETUP_TOKEN"
	corsEnvVar        = "SUBCONVERTER_CORS"

	serverReadHeaderTimeout = 10 * time.Second
	serverIdleTimeout       = 120 * time.Second
)

func main() {
	configPath := flag.String("config", "", "path to YAML config file (required)")
	listen := flag.String("listen", "", "listen address (overrides SUBCONVERTER_LISTEN)")
	accessToken := flag.String("access-token", "", "access token for /generate (overrides SUBCONVERTER_TOKEN)")
	authState := flag.String("auth-state", "", "path to admin auth state (overrides SUBCONVERTER_AUTH_STATE)")
	setupToken := flag.String("setup-token", "", "bootstrap token for first admin setup (overrides SUBCONVERTER_SETUP_TOKEN)")
	enableCORS := flag.Bool("cors", false, "enable localhost-only development CORS (overrides SUBCONVERTER_CORS)")
	cacheTTL := flag.Duration("cache-ttl", 5*time.Minute, "subscription and template cache TTL")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP fetch timeout for subscriptions")
	showVersion := flag.Bool("version", false, "print version information and exit")
	healthcheck := flag.Bool("healthcheck", false, "check service health and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("subconverter %s\ncommit: %s\nbuilt: %s\n", version, commit, date)
		return
	}

	listenAddr := resolveListenAddress(*listen)
	resolvedAccessToken := resolveAccessToken(*accessToken)
	resolvedSetupToken := resolveSetupToken(*setupToken)
	resolvedCORS := resolveCORS(*enableCORS, flagWasSet("cors"))

	if *healthcheck {
		os.Exit(runHealthcheck(listenAddr))
	}

	if *configPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("starting subconverter %s (commit=%s built=%s)", version, commit, date)

	// Build dependency chain: HTTPFetcher -> CachedFetcher.
	httpFetcher := &fetch.HTTPFetcher{Client: &http.Client{Timeout: *timeout}}
	cachedFetcher := fetch.NewCachedFetcher(httpFetcher, *cacheTTL)

	resolvedAuthState, err := resolveAuthStatePath(*authState)
	if err != nil {
		log.Fatalf("failed to resolve auth state path: %v", err)
	}

	// Load and validate config at startup (fail-fast).
	appSvc, err := app.New(context.Background(), app.Options{
		ConfigLocation: *configPath,
		ListenAddr:     listenAddr,
		Fetcher:        cachedFetcher,
		Generate:       generate.Options{AccessToken: resolvedAccessToken},
		Version:        version,
		Commit:         commit,
		BuildDate:      date,
	})
	if err != nil {
		log.Fatalf("failed to initialize app service: %v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  resolvedAuthState,
		SetupToken: resolvedSetupToken,
	})
	if err != nil {
		log.Fatalf("failed to initialize auth service: %v", err)
	}

	// Start HTTP server.
	adminHandler := admin.New(appSvc, authSvc)
	srv := server.New(appSvc, server.Options{
		AccessToken:  resolvedAccessToken,
		AdminHandler: adminHandler,
		AdminSessionValidator: func(r *http.Request) bool {
			cookie, err := r.Cookie(auth.SessionCookieName)
			return err == nil && authSvc.IsSessionValid(cookie.Value)
		},
		EnableCORS:     resolvedCORS,
		WebFS:          webui.FS(),
		RequestCounter: appSvc.IncrementRequestCount,
	})
	httpServer := newHTTPServer(listenAddr, srv.Handler())

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on %s", listenAddr)
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

func resolveListenAddress(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue := os.Getenv(listenEnvVar); envValue != "" {
		return envValue
	}
	return defaultListenAddr
}

func resolveAccessToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(accessTokenEnvVar)
}

func resolveSetupToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(setupTokenEnvVar)
}

func resolveCORS(flagValue bool, flagSet bool) bool {
	if flagSet {
		return flagValue
	}
	envValue := strings.ToLower(strings.TrimSpace(os.Getenv(corsEnvVar)))
	return envValue == "1" || envValue == "true" || envValue == "yes"
}

func flagWasSet(name string) bool {
	wasSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			wasSet = true
		}
	})
	return wasSet
}

func resolveAuthStatePath(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if envValue := os.Getenv(authStateEnvVar); envValue != "" {
		return envValue, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "subconverter", "auth.json"), nil
}

// 仅设 ReadHeaderTimeout + IdleTimeout：前者防 slowloris，后者回收闲置 keepalive。
// 不设 WriteTimeout / ReadTimeout：/generate 响应时长受上游顺序拉取 + 渲染影响，
// 硬上限会误杀合法慢请求；GET 端点无请求 body，ReadTimeout 对抗面也为空。
func newHTTPServer(listen string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		IdleTimeout:       serverIdleTimeout,
	}
}

func runHealthcheck(listen string) int {
	target, err := healthcheckURL(listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: invalid listen address %q: %v\n", listen, err)
		return 1
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

func healthcheckURL(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", err
	}

	host = healthcheckHost(host)
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, port),
		Path:   "/healthz",
	}).String(), nil
}

func healthcheckHost(host string) string {
	switch host {
	case "", "0.0.0.0":
		return "127.0.0.1"
	case "::":
		return "::1"
	default:
		return host
	}
}
