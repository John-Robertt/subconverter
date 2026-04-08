package main

import "testing"

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
