package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-SRC-LOCAL-001: all input forms share parsing, ordering and unique IDs across source kinds.
func TestSource_LocalNodePools(t *testing.T) {
	const snell = "HK = snell, hk.example.com, 443, psk=pw, version=4\n"
	vless := vlessURIFor("HK", "hk.example.com", 443) + "\n"
	for _, mode := range []string{"inline", "file", "URL"} {
		t.Run(mode, func(t *testing.T) {
			sources := config.PreparedSources{FetchOrder: []string{"vless", "subscriptions", "snell"}, Subscriptions: []config.Source{{Content: "anytls://pw@hk.example.com:443#HK"}}}
			f := &fakeFetcher{responses: map[string][]byte{}}
			for kind, body := range map[string]string{"snell": snell, "vless": vless} {
				source := config.Source{Name: "相同来源名", Content: body}
				switch mode {
				case "file":
					location := filepath.Join(t.TempDir(), kind+".txt")
					if err := os.WriteFile(location, []byte(body), 0o600); err != nil {
						t.Fatal(err)
					}
					source.Content, source.ContentFile = "", location
				case "URL":
					location := "https://example.com/" + kind
					source.Content, source.URL = "", location
					f.responses[location] = []byte(body)
				}
				if kind == "snell" {
					sources.Snell = []config.Source{source}
				} else {
					sources.VLess = []config.Source{source}
				}
			}
			result, err := sourcePrepared(context.Background(), sources, f)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Proxies) != 3 {
				t.Fatalf("nodes=%d want 3", len(result.Proxies))
			}
			for i, kind := range []model.ProxyKind{model.KindVLess, model.KindSubscription, model.KindSnell} {
				if got := result.Proxies[i]; got.ID != model.ProxyID(i+1) || got.Kind != kind || got.Name != "HK" {
					t.Fatalf("node %d=%+v want ID=%d kind=%s name=HK", i, got, i+1, kind)
				}
			}
		})
	}
}

// T-SRC-LOCAL-002: inline/file failures are strict and identify the original physical line.
func TestSource_LocalNodePoolFailures(t *testing.T) {
	for _, kind := range []string{"snell", "vless"} {
		for _, mode := range []string{"inline", "file"} {
			for _, empty := range []bool{false, true} {
				t.Run(kind+"/"+mode+"/"+map[bool]string{true: "empty", false: "bad line"}[empty], func(t *testing.T) {
					body := "# header\n\ninvalid node\n"
					if empty {
						body = "# header\n\n// comment\n"
					}
					source := config.Source{Content: body}
					location := "sources." + kind + "[0].content"
					if mode == "file" {
						location = filepath.Join(t.TempDir(), "nodes.txt")
						if err := os.WriteFile(location, []byte(body), 0o600); err != nil {
							t.Fatal(err)
						}
						source.Content, source.ContentFile = "", location
					}
					inputs := config.PreparedSources{}
					if kind == "snell" {
						inputs.Snell = []config.Source{source}
					} else {
						inputs.VLess = []config.Source{source}
					}
					_, err := sourcePrepared(context.Background(), inputs, nil)
					if empty {
						var fetchErr *errtype.FetchError
						if !errors.As(err, &fetchErr) || fetchErr.Code != errtype.CodeFetchSubscriptionEmpty || fetchErr.URL != location {
							t.Fatalf("want located empty-source error, got=%v", err)
						}
					} else {
						var buildErr *errtype.BuildError
						if !errors.As(err, &buildErr) || !strings.Contains(buildErr.Message, location) || !strings.Contains(buildErr.Message, "第 3 行") {
							t.Fatalf("want located physical-line error, got=%v", err)
						}
					}
				})
			}
		}
	}
}
