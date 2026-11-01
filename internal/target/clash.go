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
		emptyReasonClause: "被不兼容协议或其他格式的原生节点级联清空",
	}
	filtered, err := filterUnsupportedProxies(p, []string{"snell"}, opts)
	if err != nil {
		return nil, err
	}
	return projectTargetPipeline(filtered, opts)
}
