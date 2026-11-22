package pipeline

import (
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-SNELL-001: 解析合法的 Snell Surge 行（基础写法、ShadowTLS、带空白的变体）。
func TestParseSnellSurgeLine_Valid(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantName string
		wantHost string
		wantPort int
		wantKind model.ProxyKind
		wantType string
		params   map[string]string
	}{
		{
			name:     "basic v4 with reuse/tfo",
			line:     "HK = snell, 1.2.3.4, 57891, psk=xxx, version=4, reuse=true, tfo=true",
			wantName: "HK",
			wantHost: "1.2.3.4",
			wantPort: 57891,
			wantKind: model.KindSnell,
			wantType: "snell",
			params: map[string]string{
				"psk":     "xxx",
				"version": "4",
				"reuse":   "true",
				"tfo":     "true",
			},
		},
		{
			name:     "jinqians style with spaces around '='",
			line:     "SG = snell, 5.6.7.8, 8989, psk = yyy, version = 4, reuse = true, tfo = true",
			wantName: "SG",
			wantHost: "5.6.7.8",
			wantPort: 8989,
			wantKind: model.KindSnell,
			wantType: "snell",
			params: map[string]string{
				"psk":     "yyy",
				"version": "4",
				"reuse":   "true",
				"tfo":     "true",
			},
		},
		{
			name:     "with ShadowTLS",
			line:     "JP = snell, 9.10.11.12, 443, psk=zzz, version=4, shadow-tls-password=sss, shadow-tls-sni=www.microsoft.com, shadow-tls-version=3",
			wantName: "JP",
			wantHost: "9.10.11.12",
			wantPort: 443,
			wantKind: model.KindSnell,
			wantType: "snell",
			params: map[string]string{
				"psk":                 "zzz",
				"version":             "4",
				"shadow-tls-password": "sss",
				"shadow-tls-sni":      "www.microsoft.com",
				"shadow-tls-version":  "3",
			},
		},
		{
			name:     "with obfs",
			line:     "US = snell, 10.20.30.40, 8443, psk=ppp, version=3, obfs=http, obfs-host=bing.com",
			wantName: "US",
			wantHost: "10.20.30.40",
			wantPort: 8443,
			wantKind: model.KindSnell,
			wantType: "snell",
			params: map[string]string{
				"psk":       "ppp",
				"version":   "3",
				"obfs":      "http",
				"obfs-host": "bing.com",
			},
		},
		{
			name:     "CJK name",
			line:     "香港-01 = snell, hk.example.com, 443, psk=abc, version=4",
			wantName: "香港-01",
			wantHost: "hk.example.com",
			wantPort: 443,
			wantKind: model.KindSnell,
			wantType: "snell",
			params: map[string]string{
				"psk":     "abc",
				"version": "4",
			},
		},
		{
			name:     "unknown key is preserved verbatim",
			line:     "X = snell, 1.1.1.1, 443, psk=abc, future-knob=42",
			wantName: "X",
			wantHost: "1.1.1.1",
			wantPort: 443,
			wantKind: model.KindSnell,
			wantType: "snell",
			params: map[string]string{
				"psk":         "abc",
				"future-knob": "42",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSnellSurgeLine(tt.line)
			if err != nil {
				t.Fatalf("意外错误：%v", err)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q，期望 %q", got.Name, tt.wantName)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %q，期望 %q", got.Type, tt.wantType)
			}
			if got.Server != tt.wantHost {
				t.Errorf("Server = %q，期望 %q", got.Server, tt.wantHost)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %d，期望 %d", got.Port, tt.wantPort)
			}
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %q，期望 %q", got.Kind, tt.wantKind)
			}
			if len(got.Params) != len(tt.params) {
				t.Errorf("Params 数量 = %d，期望 %d（实际=%v）", len(got.Params), len(tt.params), got.Params)
			}
			for k, v := range tt.params {
				if got.Params[k] != v {
					t.Errorf("Params[%q] = %q，期望 %q", k, got.Params[k], v)
				}
			}
		})
	}
}

// T-SNELL-002: 跳过的行（空行、注释）。
func TestParseSnellSurgeLine_Skip(t *testing.T) {
	skipLines := []string{
		"",
		"   ",
		"# comment",
		"# HK = snell, 1.2.3.4, 443, psk=x, version=4",
		"// SG = snell, ...",
		"\t  \t",
	}
	for _, line := range skipLines {
		_, err := ParseSnellSurgeLine(line)
		if !errors.Is(err, errSnellLineSkip) {
			t.Errorf("行 %q：期望 errSnellLineSkip，实际为 %v", line, err)
		}
	}
}

// T-SNELL-003: 非法行产生带 CodeBuildSnellLineInvalid 的 BuildError。
func TestParseSnellSurgeLine_Invalid(t *testing.T) {
	badLines := []struct {
		name string
		line string
	}{
		{"missing equals", "HK snell, 1.2.3.4, 443, psk=x"},
		{"empty name", "= snell, 1.2.3.4, 443, psk=x"},
		{"wrong type", "HK = ss, 1.2.3.4, 443, password=x"},
		{"missing port", "HK = snell, 1.2.3.4"},
		{"port out of range high", "HK = snell, 1.2.3.4, 70000, psk=x"},
		{"port out of range zero", "HK = snell, 1.2.3.4, 0, psk=x"},
		{"port not int", "HK = snell, 1.2.3.4, abc, psk=x"},
		{"empty server", "HK = snell, , 443, psk=x"},
		{"param missing equals", "HK = snell, 1.2.3.4, 443, psk=x, orphan"},
		{"param empty key", "HK = snell, 1.2.3.4, 443, psk=x, =value"},
		{"missing psk", "HK = snell, 1.2.3.4, 443, version=4"},
	}
	for _, tc := range badLines {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSnellSurgeLine(tc.line)
			if err == nil {
				t.Fatalf("期望行 %q 返回错误，实际为 nil", tc.line)
			}
			var be *errtype.BuildError
			if !errors.As(err, &be) {
				t.Fatalf("期望 BuildError，实际为 %T：%v", err, err)
			}
			if be.Code != errtype.CodeBuildSnellLineInvalid {
				t.Errorf("Code = %q，期望 %q", be.Code, errtype.CodeBuildSnellLineInvalid)
			}
			if be.Phase != "source" {
				t.Errorf("Phase = %q，期望 %q", be.Phase, "source")
			}
		})
	}
}

// T-SNELL-004: 重复键以最后一个为准（与 Surge 的宽松行为一致）。
func TestParseSnellSurgeLine_DuplicateKey(t *testing.T) {
	line := "HK = snell, 1.2.3.4, 443, psk=first, psk=second, version=4"
	got, err := ParseSnellSurgeLine(line)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if got.Params["psk"] != "second" {
		t.Errorf("Params[psk] = %q，期望 %q（后者覆盖前者）", got.Params["psk"], "second")
	}
}
