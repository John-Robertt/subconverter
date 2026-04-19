package target

import (
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ForSurge projects a format-agnostic Pipeline into the subset supported by
// Surge by removing VLESS nodes and their cascading consequences.
func ForSurge(p *model.Pipeline) (*model.Pipeline, error) {
	return filterByDroppedTypes(p, []string{"vless"}, cascadeOptions{
		formatName:        "surge",
		formatDisplayName: "Surge",
		rootLabel:         "vless",
		emptyCode:         errtype.CodeRenderSurgeFallbackEmpty,
		internalCode:      errtype.CodeRenderSurgeProjectionInvalid,
		emptyReasonClause: "被 vless 过滤级联清空",
	})
}
