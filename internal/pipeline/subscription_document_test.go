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

// T-SRC-DOC-001: URL suffixes force the corresponding structured subscription format.
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
				t.Fatalf("format = %q, want %q for URL %q", got, tt.want, tt.rawURL)
			}
		})
	}
}

// T-SRC-DOC-002: Clash YAML proxies are copied as native declarations before TXT parsing.
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
	}, config.Subscription{URL: rawURL})
	if err != nil {
		t.Fatalf("loadSubscription() error: %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("proxies count = %d, want 2: %+v", len(proxies), proxies)
	}

	first := proxies[0]
	if first.Name != "HK-YAML" || first.Type != "trojan" || first.Server != "hk.example.com" || first.Port != 443 {
		t.Fatalf("first proxy identity = %+v, want copied HK trojan identity", first)
	}
	if first.Kind != model.KindSubscription || first.Native == nil || first.Native.Format != model.NativeProxyFormatClash {
		t.Fatalf("first proxy origin = kind %q native %+v, want subscription/clash", first.Kind, first.Native)
	}
	for _, want := range []string{"password: secret", "ws-opts:", "path: /edge"} {
		if !strings.Contains(first.Native.Payload, want) {
			t.Errorf("native Clash payload missing %q:\n%s", want, first.Native.Payload)
		}
	}

	second := proxies[1]
	if second.Name != "SG-YAML" || second.Type != "wireguard" || second.Server != "" || second.Port != 0 {
		t.Fatalf("second proxy identity = %+v, want serverless native wireguard declaration", second)
	}
	second.ID = 1
	if err := model.ValidateProxyInvariant(second); err != nil {
		t.Fatalf("serverless native Clash proxy should satisfy invariant: %v", err)
	}
}

// T-SRC-DOC-003: Surge [Proxy] entries are copied and later sections are ignored.
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
	}, config.Subscription{URL: rawURL})
	if err != nil {
		t.Fatalf("loadSubscription() error: %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("proxies count = %d, want 2: %+v", len(proxies), proxies)
	}

	first := proxies[0]
	if first.Name != "HK-CONF" || first.Type != "trojan" || first.Server != "hk.example.com" || first.Port != 443 {
		t.Fatalf("first proxy identity = %+v, want copied HK trojan identity", first)
	}
	if first.Native == nil || first.Native.Format != model.NativeProxyFormatSurge {
		t.Fatalf("first proxy Native = %+v, want Surge native declaration", first.Native)
	}
	if first.Native.Payload != "trojan, hk.example.com, 443, password=secret, sni=edge.example.com" {
		t.Fatalf("payload = %q, want exact Surge declaration RHS", first.Native.Payload)
	}
	if proxies[1].Name != "SG-CONF" || proxies[1].Type != "external" {
		t.Fatalf("second proxy = %+v, want external declaration copied", proxies[1])
	}
	for _, proxy := range proxies {
		if proxy.Name == "DIRECT" {
			t.Fatalf("built-in DIRECT policy leaked into proxies: %+v", proxies)
		}
		if proxy.Name == "SHOULD-NOT-BE-A-NODE" {
			t.Fatalf("[Proxy Group] entry leaked into proxies: %+v", proxies)
		}
	}
}

// T-SRC-DOC-004: a structured suffix has priority over an otherwise valid TXT body.
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
			}, config.Subscription{URL: tt.rawURL})
			if err == nil {
				t.Fatal("expected structured-format error, got nil")
			}
			var fetchErr *errtype.FetchError
			if !errors.As(err, &fetchErr) {
				t.Fatalf("error type = %T, want *errtype.FetchError", err)
			}
			if fetchErr.Code != errtype.CodeFetchSubscriptionFormatInvalid {
				t.Fatalf("Code = %q, want %q", fetchErr.Code, errtype.CodeFetchSubscriptionFormatInvalid)
			}
			if !strings.Contains(fetchErr.Message, tt.wantFormat) {
				t.Fatalf("message = %q, want format label %q", fetchErr.Message, tt.wantFormat)
			}
			if strings.Contains(fetchErr.URL, "token=") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("structured subscription error leaked token: %v", err)
			}
		})
	}
}
