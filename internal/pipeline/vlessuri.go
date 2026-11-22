package pipeline

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// uuidRe 匹配以短横线分隔的标准 UUID（不区分大小写）。
// VLESS 不像 SS 那样有 base64 userinfo 的惯例，所以解析器只接受这种规范形式。
// 部分 V2Ray 分支接受非标准的 32 字符“伪 UUID”，这里一律拒绝，以保持解析严格。
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// vlessKnownQueryKeys 把与传输方式无关的 URI query 键映射到 Params 键。
// 键按 Clash 目标字段名存储，渲染器可以原样读取，无需再做一层改名
// （domain-model.md 中的约定）。
//
// 注意 `type` 不在这个 map 中：它作为传输方式的规范化与分派键单独处理
// （见 applyVLessQuery）。`path` / `host` / `serviceName` / `mode` 也不在这里：
// 它们与具体传输方式相关，由 applyVLessTransportQuery 分派到对应传输方式的 Params 键。
//
// 既不在这个 map 中、也不属于上面单独处理的键，会被静默忽略（宽松处理，
// 与 ssuri.go 一致）。
var vlessKnownQueryKeys = map[string]string{
	"security":   "security",
	"encryption": "encryption",
	"flow":       "flow",
	"sni":        "servername",
	"fp":         "client-fingerprint",
	"alpn":       "alpn",
	"pbk":        "reality-public-key",
	"sid":        "reality-short-id",
	"spx":        "reality-spider-x",
}

// vlessAllowedNetworks 列出本代码库为其专门处理 Clash 传输选项（transport-opts）的
// 传输方式取值。不在这个集合中的值会被规范化为 tcp（见 normalizeVLessNetwork），
// 与 Mihomo 文档描述的后备行为一致。
var vlessAllowedNetworks = map[string]bool{
	"tcp":   true,
	"ws":    true,
	"http":  true,
	"h2":    true,
	"grpc":  true,
	"xhttp": true,
}

// ParseVLessURI 把一条标准 VLESS URI 解析为 model.Proxy。
//
// 支持的形式：vless://UUID@server:port[/][?query][#NodeName]
//
// 这里内含的设计取舍：
//   - `security` 只接受 "none" / "tls" / "reality"，其他值报错。
//     理由：这是 VLESS 规范中的三种模式；其他取值意味着 Clash 客户端
//     无法连接该节点。
//   - `encryption` 非空时原样复制。理由：Mihomo 现已支持非 "none" 的 VLESS
//     加密模式，拒绝它们会让本来有效的 URI 无法使用。
//   - `type`（传输方式）按 Mihomo 的后备语义规范化到 Params["network"]：
//     已知值 {tcp,ws,http,h2,grpc,xhttp} 保留，缺失或未知的值变为 tcp。
//   - 值为空的 query 键不存储。理由：与 ssuri.go 处理空 plugin 值的方式一致，
//     减少 Params 中的无用条目。
//   - 未知的 query 键被静默忽略。理由：向前兼容未来 V2Ray/Xray 的 query 扩展。
//
// 返回的 Proxy：Kind=KindVLess，Type="vless"。
func ParseVLessURI(raw string) (model.Proxy, error) {
	const prefix = "vless://"
	if !strings.HasPrefix(raw, prefix) {
		return model.Proxy{}, vlessURIError(raw, "缺少 vless:// 前缀")
	}

	body := raw[len(prefix):]

	// Fragment → Name。必填且不能为空。
	name := ""
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		fragment := body[idx+1:]
		body = body[:idx]
		decoded, err := url.PathUnescape(fragment)
		if err != nil {
			return model.Proxy{}, vlessURIError(raw, fmt.Sprintf("fragment 编码无效：%v", err))
		}
		name = decoded
	}
	if name == "" {
		return model.Proxy{}, vlessURIError(raw, "节点名称为空")
	}

	// Query 部分。
	query := ""
	if idx := strings.Index(body, "?"); idx >= 0 {
		query = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimSuffix(body, "/")

	// 在最后一个 '@' 处拆分 Userinfo（UUID）与 hostport。fragment 里可能含有 '@'，
	// 但前面已经去掉了；这样处理是为了兼容少见分支方言中经过编码的特殊密码。
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return model.Proxy{}, vlessURIError(raw, "缺少 @ 分隔符")
	}
	userinfo := body[:atIdx]
	hostport := body[atIdx+1:]

	if !uuidRe.MatchString(userinfo) {
		return model.Proxy{}, vlessURIError(raw, fmt.Sprintf("UUID 无效 %q", userinfo))
	}

	server, portStr, err := splitHostPort(hostport)
	if err != nil {
		return model.Proxy{}, vlessURIError(raw, err.Error())
	}
	if server == "" {
		return model.Proxy{}, vlessURIError(raw, "server 为空")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return model.Proxy{}, vlessURIError(raw, fmt.Sprintf("端口 %q 不是整数", portStr))
	}
	if port < 1 || port > 65535 {
		return model.Proxy{}, vlessURIError(raw, fmt.Sprintf("端口 %d 超出 1-65535 范围", port))
	}

	params := map[string]string{
		"uuid":    userinfo,
		"network": normalizeVLessNetwork(""),
	}

	if query != "" {
		values, err := url.ParseQuery(query)
		if err != nil {
			return model.Proxy{}, vlessURIError(raw, fmt.Sprintf("query 参数无效：%v", err))
		}
		if err := applyVLessQuery(values, params); err != nil {
			return model.Proxy{}, vlessURIError(raw, err.Error())
		}
	}

	return model.Proxy{
		Name:   name,
		Type:   "vless",
		Server: server,
		Port:   port,
		Params: params,
		Kind:   model.KindVLess,
	}, nil
}

// applyVLessQuery 校验已知的 query 值并复制到 params 中。
//
// 第 1 步：读取 `type` query 确定 `network`。缺失或未知的值规范化为 tcp，
// 与 Mihomo 文档描述的后备行为一致。
// 第 2 步：按 vlessKnownQueryKeys 复制与传输方式无关的键。
// 第 3 步：把与传输方式相关的键（path/host/serviceName/mode）分派到对应
// 传输方式的 Params 键，渲染器因此无需在运行时分派，就能输出正确的 *-opts 块。
func applyVLessQuery(values url.Values, params map[string]string) error {
	network := normalizeVLessNetwork(values.Get("type"))
	params["network"] = network

	for uriKey, paramsKey := range vlessKnownQueryKeys {
		v := strings.TrimSpace(values.Get(uriKey))
		if v == "" {
			continue
		}

		switch uriKey {
		case "security":
			if v != "none" && v != "tls" && v != "reality" {
				return fmt.Errorf("security %q 不支持（仅接受 none/tls/reality）", v)
			}
		}

		params[paramsKey] = v
	}

	applyVLessTransportQuery(values, params, network)
	return nil
}

// normalizeVLessNetwork 把 URI query 中的 `type` 值映射为保存在
// Params["network"] 中的传输方式名称。Mihomo 文档规定，`type` 省略或取任何
// 未知值时默认为 tcp，所以这个函数沿用该面向客户端的约定，而不是严格报错。
func normalizeVLessNetwork(raw string) string {
	network := strings.TrimSpace(raw)
	if network == "" {
		return "tcp"
	}
	if !vlessAllowedNetworks[network] {
		return "tcp"
	}
	return network
}

// applyVLessTransportQuery 按 network 把传输层 query 键分派到对应传输方式的
// Params 条目。键名与 Clash 的 *-opts 字段一一对应（ws-opts.path、
// grpc-opts.grpc-service-name 等），渲染器因此直接查找 `<network>-<field>`。
//
// 各传输方式处理的 URI query 键：
//   - ws:    path → ws-path；host → ws-host（变为 ws-opts.headers.Host）
//   - http:  path → http-path；host → http-host（变为 http-opts.headers.Host 列表）
//   - h2:    path → h2-path；host → h2-host（变为 h2-opts.host 列表）
//   - grpc:  serviceName → grpc-service-name
//   - xhttp: path → xhttp-path；host → xhttp-host；mode → xhttp-mode
//   - tcp:   （没有传输方式专属的键）
//
// 值会去掉两侧空白；空值不存储。渲染器把缺失的传输键理解为
// “输出 opts 块，但不含该子字段”。
func applyVLessTransportQuery(values url.Values, params map[string]string, network string) {
	trimmed := func(k string) string { return strings.TrimSpace(values.Get(k)) }

	switch network {
	case "ws":
		if v := trimmed("path"); v != "" {
			params["ws-path"] = v
		}
		if v := trimmed("host"); v != "" {
			params["ws-host"] = v
		}
	case "http":
		if v := trimmed("path"); v != "" {
			params["http-path"] = v
		}
		if v := trimmed("host"); v != "" {
			params["http-host"] = v
		}
	case "h2":
		if v := trimmed("path"); v != "" {
			params["h2-path"] = v
		}
		if v := trimmed("host"); v != "" {
			params["h2-host"] = v
		}
	case "grpc":
		if v := trimmed("serviceName"); v != "" {
			params["grpc-service-name"] = v
		}
	case "xhttp":
		if v := trimmed("path"); v != "" {
			params["xhttp-path"] = v
		}
		if v := trimmed("host"); v != "" {
			params["xhttp-host"] = v
		}
		if v := trimmed("mode"); v != "" {
			params["xhttp-mode"] = v
		}
	}
}

// vlessURIError 生成一个 BuildError，其中的 URI 会截断后显示，
// 截断方式与 ssuri.go 中 ssError 的约定一致。
func vlessURIError(uri, reason string) error {
	display := uri
	if len(display) > 80 {
		display = display[:77] + "..."
	}
	return &errtype.BuildError{
		Code:    errtype.CodeBuildVLessURIInvalid,
		Phase:   "source",
		Message: fmt.Sprintf("VLESS URI %q 无效：%s", display, reason),
	}
}
