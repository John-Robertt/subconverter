package pipeline

import (
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/ssparse"
)

// ParseSSURI parses a SIP002 Shadowsocks URI into a model.Proxy.
//
// Supported form: ss://userinfo@server:port[/][?query][#NodeName]
//
// userinfo may be either:
//   - base64/base64url encoded method:password
//   - plain method:password with percent-encoding when required
//
// Query parameters are parsed according to SIP002. Unknown query parameters are
// ignored. The plugin query is preserved in a generic Plugin structure and is
// interpreted later by target renderers.
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
