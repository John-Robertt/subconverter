package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/generate"
)

const validConfigYAML = `
sources: {}
filters: {}
groups:
  HK:
    match: "(HK)"
    strategy: select
routing:
  proxy:
    - HK
    - DIRECT
rulesets: {}
rules: []
fallback: proxy
`

type mapFetcher struct {
	responses map[string][]byte
}

func (f *mapFetcher) Fetch(_ context.Context, rawURL string) ([]byte, error) {
	body, ok := f.responses[rawURL]
	if !ok {
		return nil, errors.New("missing response")
	}
	return append([]byte(nil), body...), nil
}

func TestConfigSnapshotAndSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ConfigSnapshot: %v", err)
	}
	var body struct {
		Sources struct {
			FetchOrder []string `json:"fetch_order"`
		} `json:"sources"`
		Groups []struct {
			Key string `json:"key"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(snapshot.Config, &body); err != nil {
		t.Fatalf("unmarshal config json: %v", err)
	}
	if len(body.Groups) != 1 || body.Groups[0].Key != "HK" {
		t.Fatalf("groups JSON = %+v", body.Groups)
	}
	if got := body.Sources.FetchOrder; len(got) != 3 || got[0] != "subscriptions" || got[1] != "snell" || got[2] != "vless" {
		t.Fatalf("fetch_order = %v", got)
	}

	result, err := svc.SaveConfig(context.Background(), &SaveConfigInput{
		ConfigRevision: snapshot.ConfigRevision,
		Config:         snapshot.Config,
	})
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if result.ConfigRevision == "" || result.ConfigRevision == snapshot.ConfigRevision {
		t.Fatalf("new revision = %q, old = %q", result.ConfigRevision, snapshot.ConfigRevision)
	}
}

func TestSaveConfigRevisionConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ConfigSnapshot: %v", err)
	}
	if err := os.WriteFile(path, []byte(validConfigYAML+"\n# external edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = svc.SaveConfig(context.Background(), &SaveConfigInput{
		ConfigRevision: snapshot.ConfigRevision,
		Config:         snapshot.Config,
	})
	var conflict *errtype.RevisionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %[1]v, want RevisionConflictError", err)
	}
	if conflict.CurrentConfigRevision == "" {
		t.Fatal("CurrentConfigRevision should be populated")
	}
}

func TestSaveConfigReadonlySourceAndFileNotWritable(t *testing.T) {
	remoteSvc := NewWithRuntime("https://config.example.com/config.yaml", nil, nil, generateOptions())
	_, err := remoteSvc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: "sha256:x", Config: json.RawMessage(`{}`)})
	if !errors.Is(err, errtype.ErrConfigSourceReadonly) {
		t.Fatalf("remote SaveConfig error = %T %[1]v", err)
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o400); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ConfigSnapshot: %v", err)
	}
	_, err = svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: snapshot.Config})
	if !errors.Is(err, errtype.ErrConfigFileNotWritable) {
		t.Fatalf("readonly SaveConfig error = %T %[1]v", err)
	}
}

func generateOptions() generate.Options {
	return generate.Options{}
}

func TestValidateDraftStructuredDiagnostics(t *testing.T) {
	svc := &Service{}
	result, err := svc.ValidateDraft(context.Background(), json.RawMessage(`{
		"sources": {"fetch_order":["subscriptions","subscriptions","vless"]},
		"groups": [],
		"routing": [],
		"rulesets": [],
		"rules": [],
		"fallback": ""
	}`))
	if err != nil {
		t.Fatalf("ValidateDraft: %v", err)
	}
	if result.Valid {
		t.Fatal("result.Valid = true, want false")
	}
	var hasFetchOrder bool
	for _, item := range result.Errors {
		if item.Code == "invalid_fetch_order" && item.Locator.JSONPointer == "/config/sources/fetch_order" {
			hasFetchOrder = true
		}
	}
	if !hasFetchOrder {
		t.Fatalf("diagnostics missing invalid_fetch_order: %+v", result.Errors)
	}
}

func TestReloadInvalidatesRemoteConfigCache(t *testing.T) {
	rawURL := "https://config.example.com/config.yaml"
	inner := &mapFetcher{responses: map[string][]byte{rawURL: []byte(validConfigYAML)}}
	cached := fetch.NewCachedFetcher(inner, time.Hour)
	svc, err := New(context.Background(), Options{ConfigLocation: rawURL, Fetcher: cached})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	inner.responses[rawURL] = []byte(`sources: {}`)
	_, err = svc.Reload(context.Background())
	if err == nil {
		t.Fatal("Reload error = nil, want validation error from refreshed remote config")
	}
	if result, ok := ValidateResultFromError(err); !ok || result.Valid {
		t.Fatalf("Reload error = %v, want validation diagnostics", err)
	}
}
