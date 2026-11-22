package target

import (
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ForSurge 把与格式无关的 Pipeline 投影为 Surge 支持的子集：去掉 VLESS 节点和
// Clash 原生订阅节点，并一并处理由此引起的级联后果。
func ForSurge(p *model.Pipeline) (*model.TargetPipeline, error) {
	opts := cascadeOptions{
		formatName:        "surge",
		formatDisplayName: "Surge",
		nativeFormat:      model.NativeProxyFormatSurge,
		rootLabel:         "vless",
		emptyCode:         errtype.CodeTargetSurgeFallbackEmpty,
		internalCode:      errtype.CodeTargetSurgeProjectionInvalid,
		emptyReasonClause: "被不兼容协议或其他格式的原生节点级联清空",
	}
	filtered, err := filterUnsupportedProxies(p, []string{"vless"}, opts)
	if err != nil {
		return nil, err
	}
	return projectTargetPipeline(filtered, opts)
}
