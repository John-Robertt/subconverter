package model

import (
	"strings"
	"testing"
)

// T-MDL-LOON-001: native Loon declarations use subscription invariants without normalized params.
func TestModel_LoonNativeInvariant(t *testing.T) {
	proxy := Proxy{ID: 1, Name: "HK", Type: "anytls", Kind: KindSubscription,
		Native: &NativeProxy{Format: NativeProxyFormatLoon, Payload: `anytls, node.example.com, 443, "pw"`}}
	if err := ValidateProxyInvariant(proxy); err != nil {
		t.Fatalf("native Loon invariant: %v", err)
	}
}

// T-MDL-004: valid proxy invariant cases pass validation
func TestValidateProxyInvariant_ValidCases(t *testing.T) {
	tests := []struct {
		name  string
		proxy Proxy
	}{
		{
			name: "subscription ss",
			proxy: Proxy{
				Name:   "HK-01",
				Type:   "ss",
				Server: "hk.example.com",
				Port:   8388,
				Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"},
				Kind:   KindSubscription,
			},
		},
		{
			name: "subscription anytls",
			proxy: Proxy{
				Name:   "HK-AnyTLS",
				Type:   "anytls",
				Server: "hk.example.com",
				Port:   443,
				Params: map[string]string{"password": "secret"},
				Kind:   KindSubscription,
			},
		},
		{
			name: "snell source",
			proxy: Proxy{
				Name:   "HK-Snell",
				Type:   "snell",
				Server: "1.2.3.4",
				Port:   57891,
				Params: map[string]string{"psk": "x"},
				Kind:   KindSnell,
			},
		},
		{
			name: "vless source",
			proxy: Proxy{
				Name:   "HK-VL",
				Type:   "vless",
				Server: "hk.example.com",
				Port:   443,
				Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp"},
				Kind:   KindVLess,
			},
		},
		{
			name: "custom socks5",
			proxy: Proxy{
				Name:   "MY-PROXY",
				Type:   "socks5",
				Server: "1.2.3.4",
				Port:   1080,
				Kind:   KindCustom,
			},
		},
		{
			name: "chained ss",
			proxy: Proxy{
				Name:            "HK-01→CHAIN",
				Type:            "ss",
				Server:          "1.2.3.4",
				Port:            8388,
				Params:          map[string]string{"cipher": "aes-256-gcm", "password": "pw"},
				Kind:            KindChained,
				DialerID:        2,
				ChainNameSuffix: "CHAIN",
			},
		},
		{
			name: "native clash subscription with arbitrary type",
			proxy: Proxy{
				Name: "HK-Native",
				Type: "wireguard",
				Kind: KindSubscription,
				Native: &NativeProxy{
					Format:  NativeProxyFormatClash,
					Payload: "name: HK-Native\ntype: wireguard\n",
				},
			},
		},
		{
			name: "native surge subscription without endpoint",
			proxy: Proxy{
				Name: "SG-External",
				Type: "external",
				Kind: KindSubscription,
				Native: &NativeProxy{
					Format:  NativeProxyFormatSurge,
					Payload: `external, exec="/opt/proxy"`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := tt.proxy
			if proxy.ID == 0 {
				proxy.ID = 1
			}
			if err := ValidateProxyInvariant(proxy); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// T-MDL-005: invalid proxy invariant cases return errors
func TestValidateProxyInvariant_InvalidCases(t *testing.T) {
	tests := []struct {
		name       string
		proxy      Proxy
		wantSubstr string
	}{
		{
			name: "unsupported kind-type pair",
			proxy: Proxy{
				Name:   "HK-01",
				Type:   "socks5",
				Server: "1.2.3.4",
				Port:   1080,
				Kind:   KindSubscription,
			},
			wantSubstr: `kind="subscription" 不能使用 type="socks5"`,
		},
		{
			name: "non chained has dialer",
			proxy: Proxy{
				Name:     "MY-PROXY",
				Type:     "socks5",
				Server:   "1.2.3.4",
				Port:     1080,
				Kind:     KindCustom,
				DialerID: 2,
			},
			wantSubstr: "仅 kind=chained 允许设置 dialer_id",
		},
		{
			name: "chained missing dialer",
			proxy: Proxy{
				Name:   "HK-01→CHAIN",
				Type:   "socks5",
				Server: "1.2.3.4",
				Port:   1080,
				Kind:   KindChained,
			},
			wantSubstr: "kind=chained 时 dialer_id 必填",
		},
		{
			name: "ss missing password",
			proxy: Proxy{
				Name:   "HK-01",
				Type:   "ss",
				Server: "hk.example.com",
				Port:   8388,
				Params: map[string]string{"cipher": "aes-256-gcm"},
				Kind:   KindSubscription,
			},
			wantSubstr: "type=ss 缺少必填参数 password",
		},
		{
			name: "anytls missing password",
			proxy: Proxy{
				Name:   "HK-AnyTLS",
				Type:   "anytls",
				Server: "hk.example.com",
				Port:   443,
				Kind:   KindSubscription,
			},
			wantSubstr: "type=anytls 缺少必填参数 password",
		},
		{
			name: "vless missing network",
			proxy: Proxy{
				Name:   "HK-VL",
				Type:   "vless",
				Server: "hk.example.com",
				Port:   443,
				Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555"},
				Kind:   KindVLess,
			},
			wantSubstr: "type=vless 缺少必填参数 network",
		},
		{
			name: "native declaration on non subscription kind",
			proxy: Proxy{
				Name: "HK-Native",
				Type: "trojan",
				Kind: KindCustom,
				Native: &NativeProxy{
					Format:  NativeProxyFormatClash,
					Payload: "name: HK-Native\ntype: trojan\n",
				},
			},
			wantSubstr: "原生目标声明仅允许 kind=subscription",
		},
		{
			name: "native declaration with unknown format",
			proxy: Proxy{
				Name: "HK-Native",
				Type: "trojan",
				Kind: KindSubscription,
				Native: &NativeProxy{
					Format:  "unknown",
					Payload: "value",
				},
			},
			wantSubstr: `原生目标格式 "unknown" 不受支持`,
		},
		{
			name: "native declaration with empty payload",
			proxy: Proxy{
				Name: "HK-Native",
				Type: "trojan",
				Kind: KindSubscription,
				Native: &NativeProxy{
					Format: NativeProxyFormatClash,
				},
			},
			wantSubstr: "原生目标声明 payload 不能为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := tt.proxy
			if proxy.ID == 0 {
				proxy.ID = 1
			}
			err := ValidateProxyInvariant(proxy)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantSubstr)
			}
		})
	}
}
