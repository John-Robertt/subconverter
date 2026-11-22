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
			t.Fatalf("resolveListenAddress() = %q，期望 %q", got, defaultListenAddr)
		}
	})

	t.Run("uses env when flag is empty", func(t *testing.T) {
		t.Setenv(listenEnvVar, ":9090")

		if got := resolveListenAddress(""); got != ":9090" {
			t.Fatalf("resolveListenAddress() = %q，期望 %q", got, ":9090")
		}
	})

	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv(listenEnvVar, ":9090")

		if got := resolveListenAddress(":7070"); got != ":7070" {
			t.Fatalf("resolveListenAddress() = %q，期望 %q", got, ":7070")
		}
	})
}

func TestResolveAccessToken(t *testing.T) {
	t.Run("uses empty value when flag and env are empty", func(t *testing.T) {
		t.Setenv(accessTokenEnvVar, "")

		if got := resolveAccessToken(""); got != "" {
			t.Fatalf("resolveAccessToken() = %q，期望空字符串", got)
		}
	})

	t.Run("uses env when flag is empty", func(t *testing.T) {
		t.Setenv(accessTokenEnvVar, "env-token")

		if got := resolveAccessToken(""); got != "env-token" {
			t.Fatalf("resolveAccessToken() = %q，期望 %q", got, "env-token")
		}
	})

	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv(accessTokenEnvVar, "env-token")

		if got := resolveAccessToken("flag-token"); got != "flag-token" {
			t.Fatalf("resolveAccessToken() = %q，期望 %q", got, "flag-token")
		}
	})
}

func TestResolveCORS(t *testing.T) {
	t.Run("env enables when flag unset", func(t *testing.T) {
		t.Setenv(corsEnvVar, "true")

		if got := resolveCORS(false, false); !got {
			t.Fatal("resolveCORS() = false，期望 true")
		}
	})

	t.Run("explicit false flag overrides env", func(t *testing.T) {
		t.Setenv(corsEnvVar, "true")

		if got := resolveCORS(false, true); got {
			t.Fatal("resolveCORS() = true，期望 false")
		}
	})

	t.Run("explicit true flag enables", func(t *testing.T) {
		t.Setenv(corsEnvVar, "")

		if got := resolveCORS(true, true); !got {
			t.Fatal("resolveCORS() = false，期望 true")
		}
	})
}

// 断言威胁模型而非常量值：ReadHeaderTimeout 防 slowloris、IdleTimeout 回收 keepalive；
// WriteTimeout / ReadTimeout 显式保持 0，防止被无意加回后误杀合法慢生成请求。
func TestNewHTTPServerTimeouts(t *testing.T) {
	srv := newHTTPServer(":9090", http.NewServeMux())

	if srv.Addr != ":9090" {
		t.Fatalf("Addr = %q，期望 :9090", srv.Addr)
	}
	if srv.ReadHeaderTimeout <= 0 || srv.ReadHeaderTimeout > 30*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v，期望为正数且不超过防 slowloris 的上限（<=30s）", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout < srv.ReadHeaderTimeout {
		t.Fatalf("IdleTimeout = %v，应当 >= ReadHeaderTimeout = %v", srv.IdleTimeout, srv.ReadHeaderTimeout)
	}
	if srv.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v，期望 0（生成响应本来就可能很慢）", srv.WriteTimeout)
	}
	if srv.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %v，期望 0（GET 端点没有请求 body）", srv.ReadTimeout)
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
				t.Fatalf("healthcheckURL(%q) 返回错误：%v", tt.listen, err)
			}
			if got != tt.want {
				t.Fatalf("healthcheckURL(%q) = %q，期望 %q", tt.listen, got, tt.want)
			}
		})
	}
}

func TestHealthcheckURLRejectsInvalidListenAddress(t *testing.T) {
	if _, err := healthcheckURL("8080"); err == nil {
		t.Fatal("healthcheckURL() error = nil，期望非 nil")
	}
}

// T-CLI-STORE-001：flag 覆盖环境变量，默认值为 /config。
func TestResolveDataDir(t *testing.T) {
	for _, tt := range []struct{ name, flag, env, want string }{
		{"default", "", "", "/config"}, {"environment", "", "custom/config", "custom/config"}, {"flag", "local/config", "other/config", "local/config"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(dataDirEnvVar, tt.env)
			if got := resolveDataDir(tt.flag); got != tt.want {
				t.Fatalf("得到 %q，期望 %q", got, tt.want)
			}
		})
	}
}
