package server

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/John-Robertt/subconverter/internal/generate"
)

// Generator 把生成用例从 HTTP 传输层中抽象出来。
type Generator interface {
	Generate(ctx context.Context, req generate.Request) (*generate.Result, error)
}

// Server 保存 HTTP 处理器所需的依赖。
type Server struct {
	generator Generator
	opts      Options
}

// Options 保存只在运行时生效的服务行为开关。
type Options struct {
	AccessToken           string
	AdminHandler          http.Handler
	AdminSessionValidator func(*http.Request) bool
	EnableCORS            bool
	WebFS                 fs.FS
	// RequestCounter 设置后，每个入站请求调用一次，为 /api/status 上的运行环境
	// 信息提供数据。
	RequestCounter func()
}

// New 用给定的 generator 创建 Server。
func New(generator Generator, opts Options) *Server {
	return &Server{generator: generator, opts: opts}
}

// Handler 返回注册好全部路由的 http.Handler。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	if s.opts.AdminHandler != nil {
		mux.Handle("/api/", s.opts.AdminHandler)
	}
	mux.HandleFunc("GET /generate", s.handleGenerate)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	var handler http.Handler = mux
	if s.opts.WebFS != nil {
		handler = s.webUIFallback(handler)
	}
	if s.opts.RequestCounter != nil {
		handler = requestCounterMiddleware(handler, s.opts.RequestCounter)
	}
	if s.opts.EnableCORS {
		handler = corsMiddleware(handler)
	}
	return handler
}

func requestCounterMiddleware(next http.Handler, counter func()) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 跳过 CORS 预检请求和健康探测，让计数只反映真实的客户端和管理后台流量。
		if r.Method != http.MethodOptions && r.URL.Path != "/healthz" {
			counter()
		}
		next.ServeHTTP(w, r)
	})
}
