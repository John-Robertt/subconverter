package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	calls     map[string]int
}

func (f *mapFetcher) Fetch(_ context.Context, rawURL string) ([]byte, error) {
	if f.calls != nil {
		f.calls[rawURL]++
	}
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
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after failed reload: %v", err)
	}
	if !status.ConfigDirty {
		t.Fatalf("failed remote reload should keep runtime old and mark dirty: %+v", status)
	}
	if status.ConfigRevision == status.RuntimeConfigRevision {
		t.Fatalf("status should expose last observed remote revision after failed reload: %+v", status)
	}
	if status.LastReload == nil || status.LastReload.Time == "" || status.LastReload.Success {
		t.Fatalf("failed reload should be recorded in last_reload: %+v", status.LastReload)
	}
}

func TestPreviewNodesRuntimeAndDraftAreIsolated(t *testing.T) {
	runtimeURL := "https://sub.example.com/runtime"
	draftURL := "https://sub.example.com/draft"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(configYAML(runtimeURL, "(过期)", "HK")), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &mapFetcher{responses: map[string][]byte{
		runtimeURL: subResponse(
			"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
			"ss://YWVzLTI1Ni1jZmI6cGFzcw@expire.example.com:8388#过期提醒",
		),
		draftURL: subResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01"),
	}}
	svc, err := New(context.Background(), Options{ConfigLocation: path, Fetcher: fetcher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	before, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status before: %v", err)
	}

	runtimePreview, err := svc.PreviewNodes(context.Background())
	if err != nil {
		t.Fatalf("PreviewNodes: %v", err)
	}
	if runtimePreview.Total != 2 || runtimePreview.ActiveCount != 1 || runtimePreview.FilteredCount != 1 {
		t.Fatalf("runtime preview = %+v, want total=2 active=1 filtered=1", runtimePreview)
	}

	draftPreview, err := svc.PreviewNodesFromDraft(context.Background(), json.RawMessage(configJSON(draftURL, "", "SG")))
	if err != nil {
		t.Fatalf("PreviewNodesFromDraft: %v", err)
	}
	if draftPreview.Total != 1 || draftPreview.Nodes[0].Name != "SG-01" {
		t.Fatalf("draft preview = %+v, want SG-01 only", draftPreview)
	}

	after, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after: %v", err)
	}
	if after.RuntimeConfigRevision != before.RuntimeConfigRevision || after.ConfigDirty != before.ConfigDirty || lastReloadTimeOf(after) != lastReloadTimeOf(before) {
		t.Fatalf("draft preview mutated status: before=%+v after=%+v", before, after)
	}
}

func TestPreviewGroupsReturnsSeparatedGroupsAndExpandedMembers(t *testing.T) {
	subURL := "https://sub.example.com/runtime"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(configYAML(subURL, "", "HK")), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &mapFetcher{responses: map[string][]byte{
		subURL: subResponse(
			"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
			"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk2.example.com:8388#HK-02",
		),
	}}
	svc, err := New(context.Background(), Options{ConfigLocation: path, Fetcher: fetcher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := svc.PreviewGroups(context.Background())
	if err != nil {
		t.Fatalf("PreviewGroups: %v", err)
	}
	if len(result.NodeGroups) != 1 || result.NodeGroups[0].Name != "HK" {
		t.Fatalf("NodeGroups = %+v, want HK", result.NodeGroups)
	}
	if len(result.ChainedGroups) != 1 || result.ChainedGroups[0].Name != "SS-Chain" {
		t.Fatalf("ChainedGroups = %+v, want SS-Chain", result.ChainedGroups)
	}
	if len(result.ServiceGroups) != 1 {
		t.Fatalf("ServiceGroups = %+v, want one group", result.ServiceGroups)
	}
	var hasAllExpanded bool
	for _, member := range result.ServiceGroups[0].ExpandedMembers {
		if member.Value == "HK-01" && member.Origin == "all_expanded" {
			hasAllExpanded = true
		}
	}
	if !hasAllExpanded {
		t.Fatalf("ExpandedMembers missing all_expanded HK-01: %+v", result.ServiceGroups[0].ExpandedMembers)
	}
}

func TestGenerateFromDraftDoesNotMutateRuntimeStatus(t *testing.T) {
	runtimeURL := "https://sub.example.com/runtime"
	draftURL := "https://sub.example.com/draft"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(configYAML(runtimeURL, "", "HK")), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &mapFetcher{responses: map[string][]byte{
		runtimeURL: subResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01"),
		draftURL:   subResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01"),
	}}
	svc, err := New(context.Background(), Options{ConfigLocation: path, Fetcher: fetcher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	before, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status before: %v", err)
	}

	result, err := svc.GenerateFromDraft(context.Background(), generate.Request{Format: "clash", Filename: "draft.yaml"}, json.RawMessage(configJSON(draftURL, "", "SG")))
	if err != nil {
		t.Fatalf("GenerateFromDraft: %v", err)
	}
	if !strings.Contains(string(result.Body), "SG-01") || strings.Contains(string(result.Body), "HK-01") {
		t.Fatalf("draft generate body should use draft only: %s", result.Body)
	}

	after, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after: %v", err)
	}
	if after.RuntimeConfigRevision != before.RuntimeConfigRevision || after.ConfigDirty != before.ConfigDirty || lastReloadTimeOf(after) != lastReloadTimeOf(before) {
		t.Fatalf("draft generate mutated status: before=%+v after=%+v", before, after)
	}
}

func TestStatusLocalRehashesAndRemoteDoesNotFetch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, err := New(context.Background(), Options{
		ConfigLocation: path,
		Version:        "2.0.0",
		Commit:         "abc123",
		BuildDate:      "2026-05-03",
		Now:            func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New local: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(validConfigYAML, "strategy: select", "strategy: select # same size not required", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status local: %v", err)
	}
	if !status.ConfigDirty || status.ConfigRevision == status.RuntimeConfigRevision {
		t.Fatalf("local status should rehash and become dirty: %+v", status)
	}
	if status.Version != "2.0.0" || status.ConfigSource.Type != "local" || !status.Capabilities.ConfigWrite {
		t.Fatalf("local status metadata = %+v", status)
	}

	remoteURL := "https://config.example.com/config.yaml"
	remoteFetcher := &mapFetcher{
		responses: map[string][]byte{remoteURL: []byte(validConfigYAML)},
		calls:     map[string]int{},
	}
	remoteSvc, err := New(context.Background(), Options{ConfigLocation: remoteURL, Fetcher: remoteFetcher})
	if err != nil {
		t.Fatalf("New remote: %v", err)
	}
	if _, err := remoteSvc.Status(context.Background()); err != nil {
		t.Fatalf("Status remote: %v", err)
	}
	if remoteFetcher.calls[remoteURL] != 1 {
		t.Fatalf("remote status fetched config; calls = %d, want startup only", remoteFetcher.calls[remoteURL])
	}
}

func TestGenerateLinkUsesServerTokenAndBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("base_url: https://example.com\n"+validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{
		ConfigLocation: path,
		Generate:       generate.Options{AccessToken: "server-token"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := svc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "surge", Filename: "phone.conf", IncludeToken: true})
	if err != nil {
		t.Fatalf("GenerateLink: %v", err)
	}
	want := "https://example.com/generate?format=surge&token=server-token&filename=phone.conf"
	if result.URL != want || !result.TokenIncluded {
		t.Fatalf("GenerateLink = %+v, want %s with token", result, want)
	}
	publicLink, err := svc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "surge", Filename: "phone.conf", IncludeToken: false})
	if err != nil {
		t.Fatalf("GenerateLink include_token=false: %v", err)
	}
	if publicLink.TokenIncluded || strings.Contains(publicLink.URL, "token=") {
		t.Fatalf("GenerateLink include_token=false = %+v, want no token", publicLink)
	}

	noTokenSvc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New without token: %v", err)
	}
	noToken, err := noTokenSvc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "clash", Filename: "clash.yaml", IncludeToken: true})
	if err != nil {
		t.Fatalf("GenerateLink without token: %v", err)
	}
	if noToken.TokenIncluded || strings.Contains(noToken.URL, "token=") {
		t.Fatalf("GenerateLink without server token = %+v, want token_included=false and no token query", noToken)
	}

	noBasePath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(noBasePath, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	noBaseSvc, err := New(context.Background(), Options{ConfigLocation: noBasePath})
	if err != nil {
		t.Fatalf("New without base_url: %v", err)
	}
	_, err = noBaseSvc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "surge", Filename: "phone.conf", IncludeToken: true})
	var badReq *BadRequestError
	if !errors.As(err, &badReq) || badReq.Code != "base_url_required" {
		t.Fatalf("GenerateLink without base_url error = %T %[1]v, want base_url_required", err)
	}
}

func subResponse(uris ...string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(strings.Join(uris, "\n"))))
}

func configYAML(subURL, exclude, groupName string) string {
	return `
sources:
  subscriptions:
    - url: "` + subURL + `"
  custom_proxies:
    - name: SS-Chain
      url: "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388"
      relay_through:
        type: all
        strategy: select
filters:
  exclude: "` + exclude + `"
groups:
  ` + groupName + `:
    match: "(` + groupName + `)"
    strategy: select
routing:
  proxy:
    - ` + groupName + `
    - SS-Chain
    - "@all"
    - DIRECT
rulesets: {}
rules: []
fallback: proxy
`
}

func configJSON(subURL, exclude, groupName string) string {
	return `{
  "sources": {
    "subscriptions": [{"url":"` + subURL + `"}],
    "custom_proxies": [{
      "name": "SS-Chain",
      "url": "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388",
      "relay_through": {"type":"all","strategy":"select"}
    }]
  },
  "filters": {"exclude":"` + exclude + `"},
  "groups": [{"key":"` + groupName + `","value":{"match":"(` + groupName + `)","strategy":"select"}}],
  "routing": [{"key":"proxy","value":["` + groupName + `","SS-Chain","@all","DIRECT"]}],
  "rulesets": [],
  "rules": [],
  "fallback": "proxy"
}`
}

func lastReloadTimeOf(status *StatusResult) string {
	if status == nil || status.LastReload == nil {
		return ""
	}
	return status.LastReload.Time
}
