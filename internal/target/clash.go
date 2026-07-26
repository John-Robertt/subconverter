package target

import (
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ForClash projects a format-agnostic Pipeline into the subset supported by
// Clash Meta by removing Snell and Surge-native subscription nodes together
// with their cascading consequences.
func ForClash(p *model.Pipeline) (*model.TargetPipeline, error) {
	opts := cascadeOptions{
		formatName:        "clash",
		formatDisplayName: "Clash",
		nativeFormat:      model.NativeProxyFormatClash,
		rootLabel:         "snell",
		emptyCode:         errtype.CodeTargetClashFallbackEmpty,
		internalCode:      errtype.CodeTargetClashProjectionInvalid,
		emptyReasonClause: "被 snell 过滤级联清空，或被 Surge CONF 专属节点级联清空",
	}
	filtered, err := filterUnsupportedProxies(p, []string{"snell"}, opts)
	if err != nil {
		return nil, err
	}
	return projectTargetNames(filtered, opts)
}
