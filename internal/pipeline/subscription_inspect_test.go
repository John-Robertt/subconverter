package pipeline

import (
	"reflect"
	"testing"
)

// T-SRC-CHECK-001：识别依据内容判断格式，无法区分时保留歧义，并解析出真实节点。
func TestSubscription_Inspect(t *testing.T) {
	for _, tt := range []struct {
		name, location, body, format, want string
		inline, ambiguous, invalid         bool
	}{
		{name: "Loon conf", location: "https://example.com/sub.conf", body: "[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"\n", want: "loon"},
		{name: "Surge conf", location: "https://example.com/sub.conf", body: "[Proxy]\nHK = anytls, hk.example.com, 443, password=pw\n", want: "surge"},
		{name: "shared INI", location: "https://example.com/sub.conf", body: "[Proxy]\nHK = http, hk.example.com, 80\n", ambiguous: true},
		{name: "lcf hint", location: "https://example.com/sub.lcf", body: "[Proxy]\nHK = http, hk.example.com, 80\n", want: "loon"},
		{name: "manual override", body: "[Proxy]\nHK = http, hk.example.com, 80\n", format: "surge", want: "surge", inline: true},
		{name: "Loon fragment", body: "HK = anytls, hk.example.com, 443, \"pw\"", want: "loon", inline: true},
		{name: "text despite suffix", location: "https://example.com/sub.conf", body: "anytls://pw@hk.example.com:443#HK", want: "text"},
		{name: "Clash despite suffix", location: "https://example.com/sub.conf", body: "proxies:\n  - {name: HK, type: ss, server: hk.example.com, port: 443, cipher: aes-128-gcm, password: pw}\n", want: "clash"},
		{name: "broken YAML", body: "proxies: wrong", invalid: true, inline: true},
		{name: "empty nodes", body: "[Proxy]\n", invalid: true, inline: true},
		{name: "HTML", body: "<html>expired</html>", invalid: true, inline: true},
		{name: "invalid declaration", body: "[Proxy]\nHK =\n", invalid: true, inline: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InspectSubscription(tt.location, []byte(tt.body), tt.format, tt.inline)
			if tt.invalid {
				if err == nil {
					t.Fatal("无效订阅被识别成功")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Proxies) != 1 || got.Proxies[0].ID == 0 {
				t.Fatalf("解析出的节点=%+v", got.Proxies)
			}
			if tt.ambiguous {
				if !reflect.DeepEqual(got.Candidates, []string{"surge", "loon"}) || got.Format != "" {
					t.Fatalf("歧义结果=%+v", got)
				}
			} else if got.Format != tt.want || len(got.Candidates) != 0 {
				t.Fatalf("format=%q，期望 %q，candidates=%v", got.Format, tt.want, got.Candidates)
			}
		})
	}
}

// T-SRC-CHECK-002：内容识别不改变现有 .conf 在 auto 模式下的约定。
func TestSubscription_InspectKeepsLegacyAuto(t *testing.T) {
	body := []byte("[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"\n")
	_, format, err := parseSubscriptionBody("https://example.com/sub.conf", body, "auto", false)
	if err != nil || format != "surge" {
		t.Fatalf("原有 auto 行为发生变化：format=%q err=%v", format, err)
	}
	_, format, err = parseSubscriptionBody("https://example.com/sub.conf", []byte("anytls://pw@hk.example.com:443#HK"), "text", false)
	if err != nil || format != "text" {
		t.Fatalf("显式指定 text 失败：format=%q err=%v", format, err)
	}
}
