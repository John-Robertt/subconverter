package main

import (
	"net/http"
	"testing"
	"time"
)

func TestResolveListenAddress(t *testing.T) {
	t.Run("uses default when flag and env are empty", func(t *testing.T) {
		t.Setenv(listenEnvVar, "")

		if got := resolveListenAddress(""); got != defaultListenAddr {
			t.Fatalf("resolveListenAddress() = %q, want %q", got, defaultListenAddr)
		}
	})

	t.Run("uses env when flag is empty", func(t *testing.T) {
		t.Setenv(listenEnvVar, ":9090")

		if got := resolveListenAddress(""); got != ":9090" {
			t.Fatalf("resolveListenAddress() = %q, want %q", got, ":9090")
		}
	})

	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv(listenEnvVar, ":9090")

		if got := resolveListenAddress(":7070"); got != ":7070" {
			t.Fatalf("resolveListenAddress() = %q, want %q", got, ":7070")
		}
	})
}

func TestResolveAccessToken(t *testing.T) {
	t.Run("uses empty value when flag and env are empty", func(t *testing.T) {
		t.Setenv(accessTokenEnvVar, "")

		if got := resolveAccessToken(""); got != "" {
			t.Fatalf("resolveAccessToken() = %q, want empty string", got)
		}
	})

	t.Run("uses env when flag is empty", func(t *testing.T) {
		t.Setenv(accessTokenEnvVar, "env-token")

		if got := resolveAccessToken(""); got != "env-token" {
			t.Fatalf("resolveAccessToken() = %q, want %q", got, "env-token")
		}
	})

	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv(accessTokenEnvVar, "env-token")

		if got := resolveAccessToken("flag-token"); got != "flag-token" {
			t.Fatalf("resolveAccessToken() = %q, want %q", got, "flag-token")
		}
	})
}

// 断言威胁模型而非常量值：ReadHeaderTimeout 防 slowloris、IdleTimeout 回收 keepalive；
// WriteTimeout / ReadTimeout 显式保持 0，防止被无意加回后误杀合法慢生成请求。
func TestNewHTTPServerTimeouts(t *testing.T) {
	srv := newHTTPServer(":9090", http.NewServeMux())

	if srv.Addr != ":9090" {
		t.Fatalf("Addr = %q, want :9090", srv.Addr)
	}
	if srv.ReadHeaderTimeout <= 0 || srv.ReadHeaderTimeout > 30*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want positive and within slowloris bound (<=30s)", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout < srv.ReadHeaderTimeout {
		t.Fatalf("IdleTimeout = %v should be >= ReadHeaderTimeout = %v", srv.IdleTimeout, srv.ReadHeaderTimeout)
	}
	if srv.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want 0 (generate responses are legitimately slow)", srv.WriteTimeout)
	}
	if srv.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %v, want 0 (GET endpoint has no request body)", srv.ReadTimeout)
	}
}

func TestHealthcheckURL(t *testing.T) {
	tests := []struct {
		name   string
		listen string
		want   string
	}{
		{name: "default wildcard", listen: ":8080", want: "http://127.0.0.1:8080/healthz"},
		{name: "ipv4 wildcard", listen: "0.0.0.0:8080", want: "http://127.0.0.1:8080/healthz"},
		{name: "ipv6 wildcard", listen: "[::]:8080", want: "http://[::1]:8080/healthz"},
		{name: "explicit localhost", listen: "localhost:8080", want: "http://localhost:8080/healthz"},
		{name: "explicit host", listen: "127.0.0.1:8080", want: "http://127.0.0.1:8080/healthz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := healthcheckURL(tt.listen)
			if err != nil {
				t.Fatalf("healthcheckURL(%q) returned error: %v", tt.listen, err)
			}
			if got != tt.want {
				t.Fatalf("healthcheckURL(%q) = %q, want %q", tt.listen, got, tt.want)
			}
		})
	}
}

func TestHealthcheckURLRejectsInvalidListenAddress(t *testing.T) {
	if _, err := healthcheckURL("8080"); err == nil {
		t.Fatal("healthcheckURL() error = nil, want non-nil")
	}
}
