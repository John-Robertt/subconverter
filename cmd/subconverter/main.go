// Command subconverter 运行生成代理配置的 HTTP 服务。
//
// 用法：
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
	//nolint:gosec // 这是环境变量名，不是内嵌的凭据。
	accessTokenEnvVar = "SUBCONVERTER_TOKEN"
	dataDirEnvVar     = "SUBCONVERTER_DATA_DIR"
	authStateEnvVar   = "SUBCONVERTER_AUTH_STATE"
	//nolint:gosec // 这是环境变量名，不是内嵌的凭据。
	setupTokenEnvVar = "SUBCONVERTER_SETUP_TOKEN"
	corsEnvVar       = "SUBCONVERTER_CORS"

	serverReadHeaderTimeout = 10 * time.Second
	serverIdleTimeout       = 120 * time.Second
)

func main() {
	configPath := flag.String("config", "", "YAML 配置文件路径（必填）")
	listen := flag.String("listen", "", "监听地址（覆盖 SUBCONVERTER_LISTEN）")
	accessToken := flag.String("access-token", "", "/generate 的访问令牌（覆盖 SUBCONVERTER_TOKEN）")
	dataDir := flag.String("data-dir", "", "存放可编辑内容的目录（覆盖 SUBCONVERTER_DATA_DIR；默认 /config）")
	authState := flag.String("auth-state", "", "管理后台认证状态文件路径（覆盖 SUBCONVERTER_AUTH_STATE）")
	setupToken := flag.String("setup-token", "", "首次初始化管理员时使用的引导令牌（覆盖 SUBCONVERTER_SETUP_TOKEN）")
	enableCORS := flag.Bool("cors", false, "启用仅限 localhost 的开发用 CORS（覆盖 SUBCONVERTER_CORS）")
	cacheTTL := flag.Duration("cache-ttl", 5*time.Minute, "订阅和模板的缓存 TTL")
	timeout := flag.Duration("timeout", 30*time.Second, "拉取订阅的 HTTP 超时时间")
	showVersion := flag.Bool("version", false, "打印版本信息后退出")
	healthcheck := flag.Bool("healthcheck", false, "检查服务健康状态后退出")
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

	log.Printf("正在启动 subconverter %s (commit=%s built=%s)", version, commit, date)

	// 组装依赖链：HTTPFetcher -> CachedFetcher。
	httpFetcher := &fetch.HTTPFetcher{Client: &http.Client{Timeout: *timeout}}
	cachedFetcher := fetch.NewCachedFetcher(httpFetcher, *cacheTTL)

	resolvedAuthState, err := resolveAuthStatePath(*authState)
	if err != nil {
		log.Fatalf("解析认证状态文件路径失败：%v", err)
	}

	// 加载已有配置；本地配置文件不存在时，提供需要认证的初始化界面。
	appSvc, err := app.New(context.Background(), app.Options{
		ConfigLocation: *configPath,
		DataDir:        resolveDataDir(*dataDir),
		ListenAddr:     listenAddr,
		Fetcher:        cachedFetcher,
		Generate:       generate.Options{AccessToken: resolvedAccessToken},
		Version:        version,
		Commit:         commit,
		BuildDate:      date,
	})
	if err != nil {
		log.Fatalf("初始化 app 服务失败：%v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  resolvedAuthState,
		SetupToken: resolvedSetupToken,
	})
	if err != nil {
		log.Fatalf("初始化 auth 服务失败：%v", err)
	}

	// 启动 HTTP 服务。
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

	// 收到 SIGINT / SIGTERM 时优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("正在监听 %s", listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务出错：%v", err)
		}
	}()

	<-ctx.Done()
	log.Println("正在关闭……")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("关闭出错：%v", err)
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
		fmt.Fprintf(os.Stderr, "healthcheck: 监听地址 %q 无效：%v\n", listen, err)
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
		fmt.Fprintf(os.Stderr, "healthcheck: 状态码 %d\n", resp.StatusCode)
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

func resolveDataDir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if value := os.Getenv(dataDirEnvVar); value != "" {
		return value
	}
	return app.DefaultDataDir
}
