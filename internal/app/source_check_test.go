package app

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/generate"
)

// T-APP-CHECK-005：节点池检查按行严格解析，并保留脱敏后的来源位置。
func TestApp_CheckNodePoolFailures(t *testing.T) {
	svc, _, _ := contentStoreFixture(t)
	for _, kind := range []string{"snell", "vless"} {
		location := "https://example.com/" + kind + "?token=private-pool-token"
		_, err := svc.CheckSource(context.Background(), CheckSourceInput{Kind: kind, Content: "# header\n\ninvalid node\n", SourceLocation: location})
		if err == nil || !strings.Contains(err.Error(), "第 3 行") || strings.Contains(err.Error(), "private-pool-token") {
			t.Fatalf("%s 期望严格且已脱敏的行错误，实际=%v", kind, err)
		}
	}
}

// T-APP-CHECK-001：检查返回可用的输出格式，从不保存配置，也不把 URL 换成副本。
func TestApp_CheckSource(t *testing.T) {
	svc, _, _ := contentStoreFixture(t)
	const location = "https://example.com/sub.conf?token=secret"
	const loon = "[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"\n"
	svc.fetcher = &mapFetcher{responses: map[string][]byte{location: []byte(loon)}}
	svc.generator = generate.New(svc.fetcher, generate.Options{})
	before, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name           string
		input          CheckSourceInput
		status, format string
		outputs        []string
	}{
		{"link", CheckSourceInput{Location: location}, "recognized", "loon", []string{"loon"}},
		{"body", CheckSourceInput{Content: "anytls://pw@hk.example.com:443#HK"}, "recognized", "text", []string{"clash", "surge", "loon"}},
		{"plugin compatibility", CheckSourceInput{Content: "ss://YWVzLTEyOC1nY206cHc@hk.example.com:443?plugin=v2ray-plugin%3Bmode%3Dwebsocket#HK"}, "recognized", "text", []string{"clash"}},
		{"ambiguous", CheckSourceInput{Content: "[Proxy]\nHK = http, hk.example.com, 80"}, "ambiguous", "", []string{}},
		{"corrected", CheckSourceInput{Content: "[Proxy]\nHK = http, hk.example.com, 80", Format: "surge"}, "recognized", "surge", []string{"surge"}},
		{"Snell body", CheckSourceInput{Kind: "snell", Content: "HK = snell, hk.example.com, 443, psk=pw, version=4"}, "recognized", "", []string{"surge"}},
		{"VLESS body", CheckSourceInput{Kind: "vless", Content: "vless://11111111-1111-1111-1111-111111111111@hk.example.com:443?type=tcp#HK"}, "recognized", "", []string{"clash", "loon"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CheckSource(context.Background(), tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.status || got.Format != tt.format || got.NodeCount != 1 || !reflect.DeepEqual(got.OutputFormats, tt.outputs) {
				t.Fatalf("结果=%+v", got)
			}
			if tt.input.Location != "" && got.Content != loon {
				t.Fatal("检查没有保留拉取到的副本")
			}
		})
	}
	after, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if before.ConfigRevision != after.ConfigRevision || before.RuntimeConfigRevision != after.RuntimeConfigRevision {
		t.Fatal("检查改动了已保存的配置或运行时配置")
	}
	if _, err := os.Stat(svc.contentDirectory()); !os.IsNotExist(err) {
		t.Fatalf("检查创建了版本文件：%v", err)
	}
	result, err := svc.CheckSource(context.Background(), CheckSourceInput{Location: location})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(snapshot.Config, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg["sources"] = map[string]any{"subscriptions": []any{map[string]any{"url": location, "format": result.Format}}}
	js, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: js}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	req, err := NewGenerateInput("loon", "", false)
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out.Body), "anytls, hk.example.com") {
		t.Fatal("检查过的 URL 在保存/热重载后格式变了")
	}
}

// T-APP-CHECK-002：格式错误的输入不能得到“识别成功”的结果。
func TestApp_CheckSourceFailures(t *testing.T) {
	svc, _, _ := contentStoreFixture(t)
	for _, tt := range []CheckSourceInput{{}, {Location: "a", Content: "b"}, {Content: "x", Format: "unknown"}, {Content: "<html>expired</html>"}, {Content: "proxies: wrong"}, {Content: "\x00"}, {Kind: "unknown", Content: "x"}, {Kind: "snell", Format: "surge", Content: "x"}, {Kind: "snell", Content: "# empty"}, {Kind: "vless", Content: "# empty"}} {
		if _, err := svc.CheckSource(context.Background(), tt); err == nil {
			t.Fatalf("无效输入被接受：%+v", tt)
		}
	}
}

type failedCheckFetcher struct{}

// T-APP-CHECK-004：来源提示保留按 URL 识别格式的能力，且不重新拉取已读到的正文。
func TestApp_CheckSourceSourceLocation(t *testing.T) {
	svc := NewWithRuntime("", nil, failedCheckFetcher{}, generate.Options{})
	for _, tt := range []struct {
		name, source, body, want string
		invalid                  bool
	}{
		{name: "Loon suffix", source: "https://example.com/sub.lcf?token=private", body: "[Proxy]\nHK = http, hk.example.com, 80", want: "loon"},
		{name: "content evidence wins", source: "https://example.com/sub.conf", body: "[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"", want: "loon"},
		{name: "remote fragments stay invalid", source: "https://example.com/sub.lcf", body: "HK = http, hk.example.com, 80", invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CheckSource(context.Background(), CheckSourceInput{Content: tt.body, SourceLocation: tt.source})
			if tt.invalid {
				if err == nil {
					t.Fatal("缺少 Proxy 段的远程片段被接受了")
				}
				return
			}
			if err != nil || got.Format != tt.want || got.NodeCount != 1 {
				t.Fatalf("结果=%+v err=%v，期望 format=%q 且只有一个节点", got, err, tt.want)
			}
		})
	}
}

func (failedCheckFetcher) Fetch(_ context.Context, location string) ([]byte, error) {
	return nil, &errtype.FetchError{Code: errtype.CodeFetchRequestFailed, URL: fetch.SanitizeURL(location), Message: "请求上游失败：" + location}
}

// T-APP-CHECK-003：配置只读时仍允许检查；上游错误从不暴露 URL 中的令牌。
func TestApp_CheckSourceReadonlyAndSafeError(t *testing.T) {
	svc := NewWithRuntime("https://example.com/config.yaml", nil, failedCheckFetcher{}, generate.Options{})
	if _, err := svc.CheckSource(context.Background(), CheckSourceInput{Content: "anytls://pw@hk.example.com:443#HK"}); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Capabilities.ConfigWrite || status.ConfigSource.Writable || status.ConfigSource.Type != "remote" {
		t.Fatal("检查改动了只读配置的约定")
	}
	_, err = svc.CheckSource(context.Background(), CheckSourceInput{Location: "https://example.com/sub?token=private-check-token"})
	if err == nil || strings.Contains(err.Error(), "private-check-token") {
		t.Fatalf("期望已脱敏的上游失败，实际 %v", err)
	}
}
