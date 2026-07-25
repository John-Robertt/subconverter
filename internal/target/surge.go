package target

import (
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ForSurge projects a format-agnostic Pipeline into the subset supported by
// Surge by removing VLESS and Clash-native subscription nodes together with
// their cascading consequences.
func ForSurge(p *model.Pipeline) (*model.Pipeline, error) {
	return filterUnsupportedProxies(p, []string{"vless"}, cascadeOptions{
		formatName:        "surge",
		formatDisplayName: "Surge",
		nativeFormat:      model.NativeProxyFormatSurge,
		rootLabel:         "vless",
		emptyCode:         errtype.CodeTargetSurgeFallbackEmpty,
		internalCode:      errtype.CodeTargetSurgeProjectionInvalid,
		emptyReasonClause: "被 vless 过滤级联清空，或被 Clash YAML 专属节点级联清空",
	})
}
