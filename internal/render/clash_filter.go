package render

import (
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// filterForClash returns a Pipeline view with Snell-originated proxies and
// their cascading consequences removed, so Clash Meta output never references
// Snell nodes (Clash Meta mainline does not support Snell v4/v5, which is
// the version jinqians/snell.sh produces by default).
//
// Thin wrapper over the shared cascade engine in filter_cascade.go. The
// behavior is equivalent to the pre-refactor standalone function; all Snell
// regression tests (drop path labels, fallback-empty messages, cycle guards)
// continue to pass verbatim because the Clash-specific labels (`"snell"` and
// `"被 snell 过滤级联清空"`) are injected via cascadeOptions.
func filterForClash(p *model.Pipeline) (*model.Pipeline, error) {
	return filterByDroppedTypes(p, []string{"snell"}, cascadeOptions{
		formatName:        "clash",
		formatDisplayName: "Clash",
		rootLabel:         "snell",
		emptyCode:         errtype.CodeRenderClashFallbackEmpty,
		emptyReasonClause: "被 snell 过滤级联清空",
	})
}
