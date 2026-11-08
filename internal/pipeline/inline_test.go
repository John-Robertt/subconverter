package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
)

// T-SRC-INLINE-001: native fragments, documents and URI lists share existing parsing.
func TestSource_InlineSubscription(t *testing.T) {
	for _, tt := range []struct {
		name, format, body string
		invalid            bool
	}{
		{"loon fragment", "loon", "HK = anytls, hk.example.com, 443, \"pw\"", false},
		{"surge fragment", "surge", "HK = ss, hk.example.com, 443, encrypt-method=aes-128-gcm, password=pw", false},
		{"loon document", "loon", loonSubscriptionFixture, false},
		{"clash", "clash", "proxies:\n  - {name: HK, type: ss, server: hk.example.com, port: 443, cipher: aes-128-gcm, password: pw}", false},
		{"uri", "auto", "anytls://pw@hk.example.com:443#HK", false},
		{"invalid", "loon", "[General]\npassword = do-not-log", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			proxies, err := loadSubscription(context.Background(), nil, config.Source{Content: tt.body, Format: tt.format})
			if tt.invalid {
				if err == nil || strings.Contains(err.Error(), "do-not-log") {
					t.Fatalf("want sanitized inline failure, got %v", err)
				}
				return
			}
			if err != nil || len(proxies) == 0 {
				t.Fatalf("inline parsing: count=%d err=%v", len(proxies), err)
			}
		})
	}
}

// T-SRC-INLINE-002: errors identify the inline source without exposing its body.
func TestSource_InlineFailureLocation(t *testing.T) {
	_, err := sourcePrepared(context.Background(), config.PreparedSources{Subscriptions: []config.Source{
		{Content: "anytls://pw@hk.example.com:443#HK"},
		{Format: "loon", Content: "[General]\npassword = do-not-log"},
	}}, nil)
	var sourceErr *errtype.FetchError
	if !errors.As(err, &sourceErr) || !strings.Contains(sourceErr.Message, "sources.subscriptions[1].content") {
		t.Fatalf("structured error lacks source location: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "sources.subscriptions[1].content") || strings.Contains(err.Error(), "do-not-log") {
		t.Fatalf("want source path and sanitized failure, got %v", err)
	}
}
