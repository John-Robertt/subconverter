package generate

import (
	"context"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
)

// T-GEN-INLINE-001: pasted nodes and templates generate without external resources.
func TestService_InlineNodesAndTemplates(t *testing.T) {
	for _, format := range []string{"clash", "surge", "loon"} {
		t.Run(format, func(t *testing.T) {
			cfg := validConfig(t)
			cfg.Sources.Subscriptions = []config.Source{{Content: "anytls://pw@hk.example.com:443#HK-INLINE"}}
			template := "[General]\ndns-server = 1.1.1.1\n"
			if format == "clash" {
				template = "mixed-port: 17890\n"
			}
			cfg.Templates.Inline = map[string]string{format: template}
			result, err := New(nil, Options{}).Generate(context.Background(), mustRuntimeConfig(t, cfg), Request{Format: format, Filename: DefaultFilename(format)})
			if err != nil {
				t.Fatal(err)
			}
			body := string(result.Body)
			if !strings.Contains(body, "HK-INLINE") || !strings.Contains(body, strings.TrimSpace(template)) {
				t.Fatalf("lost inline node or template: %s", body)
			}
		})
	}
}
