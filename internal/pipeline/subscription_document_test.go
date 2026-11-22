package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-SRC-DOC-001: URL 后缀强制使用对应的结构化订阅格式。
func TestSubscriptionDocumentFormatFromURL(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		want   model.NativeProxyFormat
	}{
		{name: "yaml", rawURL: "https://sub.example.com/profile.yaml?token=secret", want: model.NativeProxyFormatClash},
		{name: "yml uppercase", rawURL: "https://sub.example.com/profile.YML", want: model.NativeProxyFormatClash},
		{name: "conf", rawURL: "https://sub.example.com/profile.conf#managed", want: model.NativeProxyFormatSurge},
		{name: "txt remains content detected", rawURL: "https://sub.example.com/profile.txt", want: ""},
		{name: "extension in query ignored", rawURL: "https://sub.example.com/api?file=profile.yaml", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := subscriptionDocumentFormatFromURL(tt.rawURL); got != tt.want {
				t.Fatalf("format = %q，期望 %q，URL 为 %q", got, tt.want, tt.rawURL)
			}
		})
	}
}

// T-SRC-DOC-002: Clash YAML 中的节点在 TXT 解析之前按原生声明复制。
func TestSource_SubscriptionClashYAMLCopiesProxies(t *testing.T) {
	const rawURL = "https://sub.example.com/profile.txt"
	body := []byte(`mixed-port: 7890
proxies:
  - name: HK-YAML
    type: trojan
    server: hk.example.com
    port: 443
    password: secret
    network: ws
    ws-opts:
      path: /edge
  - { name: SG-YAML, type: wireguard, ip: 172.16.0.2, private-key: private }
`)

	proxies, err := loadSubscription(context.Background(), &fakeFetcher{
		responses: map[string][]byte{rawURL: body},
	}, config.Source{URL: rawURL})
	if err != nil {
		t.Fatalf("loadSubscription() 出错：%v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("节点数量 = %d，期望 2：%+v", len(proxies), proxies)
	}

	first := proxies[0]
	if first.Name != "HK-YAML" || first.Type != "trojan" || first.Server != "hk.example.com" || first.Port != 443 {
		t.Fatalf("第一个节点的身份信息 = %+v，期望复制出 HK trojan 的身份信息", first)
	}
	if first.Kind != model.KindSubscription || first.Native == nil || first.Native.Format != model.NativeProxyFormatClash {
		t.Fatalf("第一个节点的来源 = kind %q native %+v，期望 subscription/clash", first.Kind, first.Native)
	}
	for _, want := range []string{"password: secret", "ws-opts:", "path: /edge"} {
		if !strings.Contains(first.Native.Payload, want) {
			t.Errorf("Clash 原生声明缺少 %q：\n%s", want, first.Native.Payload)
		}
	}

	second := proxies[1]
	if second.Name != "SG-YAML" || second.Type != "wireguard" || second.Server != "" || second.Port != 0 {
		t.Fatalf("第二个节点的身份信息 = %+v，期望是不带 server 的 wireguard 原生声明", second)
	}
	second.ID = 1
	if err := model.ValidateProxyInvariant(second); err != nil {
		t.Fatalf("不带 server 的 Clash 原生节点应满足不变量：%v", err)
	}
}

// T-SRC-DOC-003: 复制 Surge [Proxy] 段的条目，忽略其后的段。
func TestSource_SubscriptionSurgeCONFCopiesProxySection(t *testing.T) {
	const rawURL = "https://sub.example.com/profile.txt"
	body := []byte("\ufeff" + `[General]
loglevel = notify

[Proxy]
DIRECT = direct
HK-CONF = trojan, hk.example.com, 443, password=secret, sni=edge.example.com
SG-CONF = external, exec="/opt/proxy,runner", args="--fast"

[Proxy Group]
SHOULD-NOT-BE-A-NODE = select, HK-CONF
`)

	proxies, err := loadSubscription(context.Background(), &fakeFetcher{
		responses: map[string][]byte{rawURL: body},
	}, config.Source{URL: rawURL})
	if err != nil {
		t.Fatalf("loadSubscription() 出错：%v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("节点数量 = %d，期望 2：%+v", len(proxies), proxies)
	}

	first := proxies[0]
	if first.Name != "HK-CONF" || first.Type != "trojan" || first.Server != "hk.example.com" || first.Port != 443 {
		t.Fatalf("第一个节点的身份信息 = %+v，期望复制出 HK trojan 的身份信息", first)
	}
	if first.Native == nil || first.Native.Format != model.NativeProxyFormatSurge {
		t.Fatalf("第一个节点的 Native = %+v，期望是 Surge 原生声明", first.Native)
	}
	if first.Native.Payload != "trojan, hk.example.com, 443, password=secret, sni=edge.example.com" {
		t.Fatalf("payload = %q，期望与 Surge 声明等号右侧完全一致", first.Native.Payload)
	}
	if proxies[1].Name != "SG-CONF" || proxies[1].Type != "external" {
		t.Fatalf("第二个节点 = %+v，期望复制出 external 声明", proxies[1])
	}
	for _, proxy := range proxies {
		if proxy.Name == "DIRECT" {
			t.Fatalf("内置 DIRECT 策略混入了节点列表：%+v", proxies)
		}
		if proxy.Name == "SHOULD-NOT-BE-A-NODE" {
			t.Fatalf("[Proxy Group] 条目混入了节点列表：%+v", proxies)
		}
	}
}

// T-SRC-DOC-004: 即使正文本身是合法的 TXT，结构化后缀仍然优先。
func TestSource_SubscriptionStructuredSuffixOverridesTXT(t *testing.T) {
	tests := []struct {
		name       string
		rawURL     string
		body       []byte
		wantFormat string
	}{
		{
			name:       "yaml suffix",
			rawURL:     "https://sub.example.com/profile.yml?token=secret",
			body:       []byte("ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01"),
			wantFormat: "Clash YAML",
		},
		{
			name:       "conf suffix",
			rawURL:     "https://sub.example.com/profile.conf?token=secret",
			body:       []byte("HK = ss, hk.example.com, 8388, encrypt-method=aes-256-gcm, password=pw"),
			wantFormat: "Surge CONF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadSubscription(context.Background(), &fakeFetcher{
				responses: map[string][]byte{tt.rawURL: tt.body},
			}, config.Source{URL: tt.rawURL})
			if err == nil {
				t.Fatal("期望返回结构化格式错误，实际为 nil")
			}
			var fetchErr *errtype.FetchError
			if !errors.As(err, &fetchErr) {
				t.Fatalf("错误类型 = %T，期望 *errtype.FetchError", err)
			}
			if fetchErr.Code != errtype.CodeFetchSubscriptionFormatInvalid {
				t.Fatalf("Code = %q，期望 %q", fetchErr.Code, errtype.CodeFetchSubscriptionFormatInvalid)
			}
			if !strings.Contains(fetchErr.Message, tt.wantFormat) {
				t.Fatalf("message = %q，期望包含格式标签 %q", fetchErr.Message, tt.wantFormat)
			}
			if strings.Contains(fetchErr.URL, "token=") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("结构化订阅错误泄露了令牌：%v", err)
			}
		})
	}
}
