package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
)

// T-SRC-INLINE-001：原生片段、完整文档和 URI 列表共用现有解析逻辑。
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
					t.Fatalf("期望脱敏后的内联来源错误，实际 %v", err)
				}
				return
			}
			if err != nil || len(proxies) == 0 {
				t.Fatalf("内联解析：count=%d err=%v", len(proxies), err)
			}
		})
	}
}

// T-SRC-INLINE-002：错误信息指明是哪个内联来源，但不暴露其内容。
func TestSource_InlineFailureLocation(t *testing.T) {
	_, err := sourcePrepared(context.Background(), config.PreparedSources{Subscriptions: []config.Source{
		{Content: "anytls://pw@hk.example.com:443#HK"},
		{Format: "loon", Content: "[General]\npassword = do-not-log"},
	}}, nil)
	var sourceErr *errtype.FetchError
	if !errors.As(err, &sourceErr) || !strings.Contains(sourceErr.Message, "sources.subscriptions[1].content") {
		t.Fatalf("结构化错误缺少来源位置：%v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "sources.subscriptions[1].content") || strings.Contains(err.Error(), "do-not-log") {
		t.Fatalf("期望包含来源路径且已脱敏的错误，实际 %v", err)
	}
}
