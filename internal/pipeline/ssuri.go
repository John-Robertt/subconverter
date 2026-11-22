package pipeline

import (
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/ssparse"
)

// ParseSSURI 把一条 SIP002 Shadowsocks URI 解析为 model.Proxy。
//
// 支持的形式：ss://userinfo@server:port[/][?query][#NodeName]
//
// userinfo 可以是以下任一种：
//   - base64/base64url 编码的 method:password
//   - 明文 method:password，必要时使用百分号编码
//
// query 参数按 SIP002 解析，未知的 query 参数被忽略。plugin 参数保存在通用的
// Plugin 结构中，稍后由各目标格式的渲染器解释。
func ParseSSURI(raw string) (model.Proxy, error) {
	const prefix = "ss://"
	if !strings.HasPrefix(raw, prefix) {
		return model.Proxy{}, ssError(raw, "缺少 ss:// 前缀")
	}

	r, err := ssparse.ParseBody(raw[len(prefix):], true)
	if err != nil {
		return model.Proxy{}, ssError(raw, err.Error())
	}

	if r.Name == "" {
		return model.Proxy{}, ssError(raw, "节点名称为空")
	}

	return model.Proxy{
		Name:   r.Name,
		Type:   "ss",
		Server: r.Server,
		Port:   r.Port,
		Params: map[string]string{
			"cipher":   r.Cipher,
			"password": r.Password,
		},
		Plugin: convertSSPlugin(r.Plugin),
		Kind:   model.KindSubscription,
	}, nil
}

func convertSSPlugin(src *ssparse.PluginSpec) *model.Plugin {
	if src == nil {
		return nil
	}
	dst := &model.Plugin{Name: src.Name}
	if len(src.Opts) > 0 {
		dst.Opts = make(map[string]string, len(src.Opts))
		for k, v := range src.Opts {
			dst.Opts[k] = v
		}
	}
	return dst
}

func ssError(uri, reason string) error {
	display := uri
	if len(display) > 80 {
		display = display[:77] + "..."
	}
	return &errtype.BuildError{
		Code:    errtype.CodeBuildSSURIInvalid,
		Phase:   "source",
		Message: fmt.Sprintf("SS URI %q 无效：%s", display, reason),
	}
}
