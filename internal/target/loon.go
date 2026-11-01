package target

import (
	"fmt"
	"strconv"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ForLoon filters incompatible proxies and allocates names for Loon chains.
func ForLoon(p *model.Pipeline) (*model.TargetPipeline, error) {
	opts := cascadeOptions{
		formatName: "loon", formatDisplayName: "Loon",
		nativeFormat:      model.NativeProxyFormatLoon,
		rootLabel:         "snell",
		emptyCode:         errtype.CodeTargetLoonFallbackEmpty,
		internalCode:      errtype.CodeTargetLoonProjectionInvalid,
		emptyReasonClause: "被不兼容协议或其他格式的原生节点级联清空",
		unsupportedProxy:  unsupportedLoonProxy,
	}
	filtered, err := filterUnsupportedProxies(p, []string{"snell"}, opts)
	if err != nil {
		return nil, err
	}
	projected, err := projectTargetPipeline(filtered, opts)
	if err != nil {
		return nil, err
	}
	return projectLoonChains(projected, opts)
}

func unsupportedLoonProxy(proxy model.Proxy) (string, bool) {
	if proxy.Type == "vless" {
		switch proxy.Params["network"] {
		case "tcp", "ws", "http":
		default:
			return fmt.Sprintf("Loon 不支持 VLESS 传输 %q", proxy.Params["network"]), true
		}
		if encryption := proxy.Params["encryption"]; encryption != "" && encryption != "none" {
			return "Loon 未适配该 VLESS encryption", true
		}
		if flow := proxy.Params["flow"]; flow != "" && flow != "xtls-rprx-vision" {
			return "Loon 未适配该 VLESS flow", true
		}
	}
	return "", false
}

func projectLoonChains(p *model.TargetPipeline, opts cascadeOptions) (*model.TargetPipeline, error) {
	reserved := make(map[string]string)
	for _, group := range p.NodeGroups {
		reserved[group.Name] = "节点组"
	}
	for _, group := range p.RouteGroups {
		reserved[group.Name] = "服务组"
	}
	reserved["DIRECT"], reserved["REJECT"] = "保留策略", "保留策略"
	for _, proxy := range p.Proxies {
		reserved[proxy.Name] = "代理或链路"
	}
	allocator := newTargetNameAllocator(reserved, opts)
	for i, proxy := range p.Proxies {
		if proxy.Dialer == "" {
			continue
		}
		exitName, err := allocator.allocate(model.ProxyID(i+1), proxy.Name+"-exit")
		if err != nil {
			return nil, err
		}
		udp, _ := strconv.ParseBool(proxy.Params["udp-relay"])
		p.ProxyChains = append(p.ProxyChains, model.TargetChain{
			Name: proxy.Name, Members: []string{proxy.Dialer, exitName}, UDP: udp,
		})
		p.Proxies[i].Name = exitName
		p.Proxies[i].Dialer = ""
	}
	return p, nil
}
