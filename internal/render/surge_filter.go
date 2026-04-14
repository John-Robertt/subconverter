package render

import (
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// filterForSurge returns a Pipeline view with VLESS-originated proxies and
// their cascading consequences removed, so Surge output never references
// VLESS nodes (Surge does not natively support VLESS — only Clash Meta does).
//
// Thin wrapper over the shared cascade engine in filter_cascade.go.
// Symmetric to filterForClash but filtering in the opposite direction:
// Clash drops Snell, Surge drops VLESS. Both use the same cascade semantics
// (drop-path labels, fallback-empty reporting, cycle guards).
func filterForSurge(p *model.Pipeline) (*model.Pipeline, error) {
	return filterByDroppedTypes(p, []string{"vless"}, cascadeOptions{
		formatName:        "surge",
		formatDisplayName: "Surge",
		rootLabel:         "vless",
		emptyCode:         errtype.CodeRenderSurgeFallbackEmpty,
		emptyReasonClause: "被 vless 过滤级联清空",
	})
}
