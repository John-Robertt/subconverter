package generate

import (
	"context"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
)

// T-GEN-LOON-001: Loon imports only nodes and uses configured groups, rules and template.
func TestService_LoonEndToEnd(t *testing.T) {
	cfg := validConfig(t)
	cfg.Sources.Subscriptions = []config.Source{{URL: testSubURL, Format: "loon"}, {URL: "https://example.com/plain.txt"}}
	cfg.Templates.Loon = "https://example.com/base.conf"
	f := &fakeFetcher{responses: map[string][]byte{
		testSubURL:                      []byte("[Proxy]\nHK-LOON = anytls, loon.example.com, 443, \"native-secret\", tls-name=edge.example.com, udp=true\n[Proxy Group]\nPROVIDER_GROUP = select, MISSING\n[Rule]\nFINAL,PROVIDER_GROUP\n[Host]\nsource-host.example.com = alias.example.com\n"),
		"https://example.com/plain.txt": []byte("anytls://pw@hk.example.com:443?sni=edge.example.com#HK-TXT"),
		"https://example.com/base.conf": []byte("[General]\ndns-server = system\n[Proxy Group]\nOLD_GROUP = select, MISSING\n[Remote Rule]\nhttps://example.com/old.list,OLD_GROUP\n[Host]\nbase-host.example.com = alias.example.com\n"),
	}}
	rt := mustRuntimeConfig(t, cfg)
	for _, format := range []string{"clash", "surge", "loon"} {
		t.Run(format, func(t *testing.T) {
			result, err := New(f, Options{}).Generate(context.Background(), rt, Request{Format: format, Filename: DefaultFilename(format)})
			if err != nil {
				t.Fatal(err)
			}
			body := string(result.Body)
			if !strings.Contains(body, "HK-TXT") {
				t.Fatalf("%s output lost shared node:\n%s", format, body)
			}
			if (strings.Contains(body, "native-secret")) != (format == "loon") {
				t.Fatalf("%s native isolation failed:\n%s", format, body)
			}
			for _, bad := range []string{"PROVIDER_GROUP", "MISSING", "source-host.example.com", "OLD_GROUP", "old.list"} {
				if strings.Contains(body, bad) {
					t.Errorf("%s output includes provider/template graph %q", format, bad)
				}
			}
			if format == "loon" {
				for _, want := range []string{"dns-server = system", "base-host.example.com = alias.example.com", "HK = select, HK-LOON, HK-TXT", "https://example.com/rules.list,proxy", "GEOIP,CN,proxy", "FINAL,proxy"} {
					if !strings.Contains(body, want) {
						t.Errorf("Loon output missing %q:\n%s", want, body)
					}
				}
				if result.Filename != "loon.lcf" || result.ContentType != "text/plain; charset=utf-8" {
					t.Fatalf("metadata=%+v", result)
				}
			}
		})
	}
}

// T-GEN-LOON-002: filename and link helpers expose the new format consistently.
func TestGenerate_LoonRequest(t *testing.T) {
	for _, tt := range []struct {
		name    string
		present bool
		want    string
		invalid bool
	}{
		{"", false, "loon.lcf", false}, {"custom", true, "custom.lcf", false},
		{"custom.lcf", true, "custom.lcf", false}, {"custom.LCF", true, "custom.LCF", false},
		{"custom.conf", true, "", true}, {"custom.yaml", true, "", true}, {".lcf", true, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveFilename(tt.name, tt.present, "loon")
			if got != tt.want || (err != nil) != tt.invalid {
				t.Fatalf("filename=%q error=%v, want %q invalid=%v", got, err, tt.want, tt.invalid)
			}
		})
	}
	url, err := BuildGenerateURL("https://example.com", "loon", DefaultFilename("loon"), "secret", true)
	if err != nil || !strings.Contains(url, "format=loon") || !strings.Contains(url, "filename=loon.lcf") || !strings.Contains(url, "token=secret") {
		t.Fatalf("url=%q error=%v", url, err)
	}
}

// T-GEN-LOON-003: 来源 URI 的指纹和链式出口账号完整保留到最终 Loon 配置。
func TestService_LoonChainCredentialsAndVLessFingerprint(t *testing.T) {
	cfg := validConfig(t)
	cfg.Sources.VLess = []config.Source{{URL: "https://example.com/vless.txt"}}
	cfg.Sources.CustomProxies = []config.CustomProxy{{
		Name: "EXIT", URL: "socks5://user:pass@exit.example.com:1080",
		RelayThrough: &config.RelayThrough{Type: "group", Name: "HK", Strategy: "select"},
	}}
	cfg.Routing = mustRoutingMap(t, `"proxy": ["HK", "EXIT", "DIRECT"]`)
	f := validFetcher()
	f.responses["https://example.com/vless.txt"] = []byte("vless://11111111-2222-3333-4444-555555555555@vless.example.com:443?security=reality&type=tcp&flow=xtls-rprx-vision&sni=edge.example.com&fp=chrome&pbk=example-public-key&sid=ab#HK-VLESS")
	result, err := New(f, Options{}).Generate(context.Background(), mustRuntimeConfig(t, cfg), Request{Format: "loon", Filename: DefaultFilename("loon")})
	if err != nil {
		t.Fatal(err)
	}
	body := string(result.Body)
	for _, want := range []string{
		`HK-VLESS = vless, vless.example.com, 443, "11111111-2222-3333-4444-555555555555", transport=tcp, flow=xtls-rprx-vision, sni=edge.example.com, public-key=example-public-key, short-id=ab, over-tls=true, tls-profile=chrome, udp=true`,
		`HK-01→EXIT-exit = socks5, exit.example.com, 1080, user, "pass"`,
		`HK-VLESS→EXIT-exit = socks5, exit.example.com, 1080, user, "pass"`,
		"[Proxy Chain]\nHK-01→EXIT = HK-01, HK-01→EXIT-exit\nHK-VLESS→EXIT = HK-VLESS, HK-VLESS→EXIT-exit",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Loon 配置缺少 %q：\n%s", want, body)
		}
	}
}
