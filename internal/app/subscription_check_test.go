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

// T-APP-CHECK-001: check returns usable outputs and never saves or switches a URL to a copy.
func TestApp_CheckSubscription(t *testing.T) {
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
		input          CheckSubscriptionInput
		status, format string
		outputs        []string
	}{
		{"link", CheckSubscriptionInput{Location: location}, "recognized", "loon", []string{"loon"}},
		{"body", CheckSubscriptionInput{Content: "anytls://pw@hk.example.com:443#HK"}, "recognized", "text", []string{"clash", "surge", "loon"}},
		{"plugin compatibility", CheckSubscriptionInput{Content: "ss://YWVzLTEyOC1nY206cHc@hk.example.com:443?plugin=v2ray-plugin%3Bmode%3Dwebsocket#HK"}, "recognized", "text", []string{"clash"}},
		{"ambiguous", CheckSubscriptionInput{Content: "[Proxy]\nHK = http, hk.example.com, 80"}, "ambiguous", "", []string{}},
		{"corrected", CheckSubscriptionInput{Content: "[Proxy]\nHK = http, hk.example.com, 80", Format: "surge"}, "recognized", "surge", []string{"surge"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CheckSubscription(context.Background(), tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.status || got.Format != tt.format || got.NodeCount != 1 || !reflect.DeepEqual(got.OutputFormats, tt.outputs) {
				t.Fatalf("result=%+v", got)
			}
			if tt.input.Location != "" && got.Content != loon {
				t.Fatal("check did not preserve the fetched copy")
			}
		})
	}
	after, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if before.ConfigRevision != after.ConfigRevision || before.RuntimeConfigRevision != after.RuntimeConfigRevision {
		t.Fatal("check changed saved config or runtime")
	}
	if _, err := os.Stat(svc.contentDirectory()); !os.IsNotExist(err) {
		t.Fatalf("check created version files: %v", err)
	}
	result, err := svc.CheckSubscription(context.Background(), CheckSubscriptionInput{Location: location})
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
		t.Fatal("checked URL changed format after save/reload")
	}
}

// T-APP-CHECK-002: malformed inputs cannot produce a successful identification badge.
func TestApp_CheckSubscriptionFailures(t *testing.T) {
	svc, _, _ := contentStoreFixture(t)
	for _, tt := range []CheckSubscriptionInput{{}, {Location: "a", Content: "b"}, {Content: "x", Format: "unknown"}, {Content: "<html>expired</html>"}, {Content: "proxies: wrong"}, {Content: "\x00"}} {
		if _, err := svc.CheckSubscription(context.Background(), tt); err == nil {
			t.Fatalf("invalid input accepted: %+v", tt)
		}
	}
}

type failedCheckFetcher struct{}

func (failedCheckFetcher) Fetch(_ context.Context, location string) ([]byte, error) {
	return nil, &errtype.FetchError{Code: errtype.CodeFetchRequestFailed, URL: fetch.SanitizeURL(location), Message: "请求上游失败：" + location}
}

// T-APP-CHECK-003: readonly config still permits checks; upstream errors never expose URL tokens.
func TestApp_CheckSubscriptionReadonlyAndSafeError(t *testing.T) {
	svc := NewWithRuntime("https://example.com/config.yaml", nil, failedCheckFetcher{}, generate.Options{})
	if _, err := svc.CheckSubscription(context.Background(), CheckSubscriptionInput{Content: "anytls://pw@hk.example.com:443#HK"}); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Capabilities.ConfigWrite || status.ConfigSource.Writable || status.ConfigSource.Type != "remote" {
		t.Fatal("inspection changed readonly config contract")
	}
	_, err = svc.CheckSubscription(context.Background(), CheckSubscriptionInput{Location: "https://example.com/sub?token=private-check-token"})
	if err == nil || strings.Contains(err.Error(), "private-check-token") {
		t.Fatalf("want sanitized upstream failure, got %v", err)
	}
}
