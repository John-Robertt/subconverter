package config

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// T-CFG-001：成功加载示例配置文件
func TestLoad_ExampleConfig(t *testing.T) {
	cfg, err := Load(context.Background(), "../../configs/base_config.yaml", nil)
	if err != nil {
		t.Fatalf("加载 base_config.yaml：%v", err)
	}

	if cfg.Fallback != "🐟 FINAL" {
		t.Errorf("Fallback = %q，期望 %q", cfg.Fallback, "🐟 FINAL")
	}
	if cfg.Groups.Len() != 5 {
		t.Errorf("Groups.Len() = %d，期望 5", cfg.Groups.Len())
	}
	if len(cfg.Sources.Subscriptions) != 1 {
		t.Errorf("Subscriptions 数量 = %d，期望 1", len(cfg.Sources.Subscriptions))
	}
	if len(cfg.Sources.CustomProxies) != 1 {
		t.Errorf("CustomProxies 数量 = %d，期望 1", len(cfg.Sources.CustomProxies))
	}

	cp := cfg.Sources.CustomProxies[0]
	if cp.Name != "🔗 HK-ISP" {
		t.Errorf("CustomProxy.Name = %q，期望 %q", cp.Name, "🔗 HK-ISP")
	}
	if cp.URL != "socks5://user:pass@154.197.1.1:45002" {
		t.Errorf("CustomProxy.URL = %q", cp.URL)
	}
	if cp.RelayThrough == nil {
		t.Fatal("RelayThrough 为 nil")
	}
	if cp.RelayThrough.Type != "group" || cp.RelayThrough.Strategy != "select" {
		t.Errorf("RelayThrough = %+v", cp.RelayThrough)
	}
}

// T-CFG-002：加载最小合法配置
func TestLoad_MinimalValid(t *testing.T) {
	cfg, err := Load(context.Background(), "../../testdata/config/minimal_valid.yaml", nil)
	if err != nil {
		t.Fatalf("加载最小配置：%v", err)
	}

	if cfg.Fallback != "final" {
		t.Errorf("Fallback = %q", cfg.Fallback)
	}
	if cfg.Groups.Len() != 1 {
		t.Errorf("Groups.Len() = %d，期望 1", cfg.Groups.Len())
	}

	hk, ok := cfg.Groups.Get("HK")
	if !ok {
		t.Fatal("未找到组 HK")
	}
	if hk.Match != "(HK)" || hk.Strategy != "select" {
		t.Errorf("HK 组 = %+v", hk)
	}
}

// T-CFG-003：文件不存在时 Load 返回错误
func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(context.Background(), "nonexistent.yaml", nil)
	if err == nil {
		t.Fatal("文件不存在时应返回错误")
	}
	var ce *errtype.ConfigError
	if !errors.As(err, &ce) {
		t.Errorf("期望 ConfigError，实际为 %T", err)
	}
}

// T-CFG-004：YAML 格式错误时 Load 返回错误
func TestLoad_MalformedYAML(t *testing.T) {
	_, err := Load(context.Background(), "../../testdata/config/malformed.yaml", nil)
	if err == nil {
		t.Fatal("YAML 格式错误时应返回错误")
	}
	var ce *errtype.ConfigError
	if !errors.As(err, &ce) {
		t.Errorf("期望 ConfigError，实际为 %T", err)
	}
}
