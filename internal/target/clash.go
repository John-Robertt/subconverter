package target

import (
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ForClash 把与格式无关的 Pipeline 投影为 Clash Meta 支持的子集：去掉 Snell
// 节点和 Surge 原生订阅节点，并一并处理由此引起的级联后果。
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
