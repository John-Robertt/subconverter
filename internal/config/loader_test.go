package config

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

func TestLoad_ExampleConfig(t *testing.T) {
	cfg, err := Load(context.Background(), "../../configs/base_config.yaml", nil)
	if err != nil {
		t.Fatalf("Load base_config.yaml: %v", err)
	}

	if cfg.Fallback != "🐟 FINAL" {
		t.Errorf("Fallback = %q, want %q", cfg.Fallback, "🐟 FINAL")
	}
	if cfg.Groups.Len() != 5 {
		t.Errorf("Groups.Len() = %d, want 5", cfg.Groups.Len())
	}
	if len(cfg.Sources.Subscriptions) != 1 {
		t.Errorf("Subscriptions count = %d, want 1", len(cfg.Sources.Subscriptions))
	}
	if len(cfg.Sources.CustomProxies) != 1 {
		t.Errorf("CustomProxies count = %d, want 1", len(cfg.Sources.CustomProxies))
	}

	cp := cfg.Sources.CustomProxies[0]
	if cp.Name != "HK-ISP" || cp.Type != "socks5" || cp.Port != 45002 {
		t.Errorf("CustomProxy = %+v", cp)
	}
	if cp.RelayThrough == nil {
		t.Fatal("RelayThrough is nil")
	}
	if cp.RelayThrough.Type != "group" || cp.RelayThrough.Strategy != "select" {
		t.Errorf("RelayThrough = %+v", cp.RelayThrough)
	}
}

func TestLoad_MinimalValid(t *testing.T) {
	cfg, err := Load(context.Background(), "../../testdata/config/minimal_valid.yaml", nil)
	if err != nil {
		t.Fatalf("Load minimal: %v", err)
	}

	if cfg.Fallback != "final" {
		t.Errorf("Fallback = %q", cfg.Fallback)
	}
	if cfg.Groups.Len() != 1 {
		t.Errorf("Groups.Len() = %d, want 1", cfg.Groups.Len())
	}

	hk, ok := cfg.Groups.Get("HK")
	if !ok {
		t.Fatal("group HK not found")
	}
	if hk.Match != "(HK)" || hk.Strategy != "select" {
		t.Errorf("HK group = %+v", hk)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(context.Background(), "nonexistent.yaml", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	var ce *errtype.ConfigError
	if !errors.As(err, &ce) {
		t.Errorf("expected ConfigError, got %T", err)
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	_, err := Load(context.Background(), "../../testdata/config/malformed.yaml", nil)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	var ce *errtype.ConfigError
	if !errors.As(err, &ce) {
		t.Errorf("expected ConfigError, got %T", err)
	}
}
