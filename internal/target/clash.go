package target

import (
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ForClash projects a format-agnostic Pipeline into the subset supported by
// Clash Meta by removing Snell nodes and their cascading consequences.
func ForClash(p *model.Pipeline) (*model.Pipeline, error) {
	return filterByDroppedTypes(p, []string{"snell"}, cascadeOptions{
		formatName:        "clash",
		formatDisplayName: "Clash",
		rootLabel:         "snell",
		emptyCode:         errtype.CodeTargetClashFallbackEmpty,
		internalCode:      errtype.CodeTargetClashProjectionInvalid,
		emptyReasonClause: "被 snell 过滤级联清空",
	})
}
