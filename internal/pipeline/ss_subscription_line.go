package pipeline

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ParseSSSubscriptionLine parses one SS subscription line in any supported
// plain-text format: SIP002 ss:// URI, Quantumult X shadowsocks line, or Surge
// proxy line.
func ParseSSSubscriptionLine(line string) (model.Proxy, error) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "ss://") {
		return ParseSSURI(trimmed)
	}
	if isQuanXShadowsocksLine(trimmed) {
		return parseQuanXShadowsocksLine(trimmed)
	}
	if isSurgeSSLine(trimmed) {
		return parseSurgeSSLine(trimmed)
	}
	return model.Proxy{}, ssSubscriptionLineError(line, "不支持的 SS 订阅行格式")
}

func isPlainSSSubscriptionLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "ss://") || isQuanXShadowsocksLine(trimmed) || isSurgeSSLine(trimmed)
}

func isQuanXShadowsocksLine(line string) bool {
	eqIdx := strings.Index(line, "=")
	return eqIdx > 0 && strings.EqualFold(strings.TrimSpace(line[:eqIdx]), "shadowsocks")
}

func isSurgeSSLine(line string) bool {
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return false
	}
	segments, err := splitSSSubscriptionSegments(line[eqIdx+1:])
	if err != nil || len(segments) == 0 {
		return false
	}
	proxyType := strings.ToLower(strings.TrimSpace(segments[0]))
	return proxyType == "ss" || proxyType == "shadowsocks"
}

func parseQuanXShadowsocksLine(line string) (model.Proxy, error) {
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return model.Proxy{}, ssSubscriptionLineError(line, "缺少 '=' 分隔符")
	}

	segments, err := splitSSSubscriptionSegments(line[eqIdx+1:])
	if err != nil {
		return model.Proxy{}, ssSubscriptionLineError(line, err.Error())
	}
	if len(segments) < 2 {
		return model.Proxy{}, ssSubscriptionLineError(line, "字段数不足，至少需要 host:port 和参数")
	}

	server, port, err := parseSSLineServerPort(segments[0])
	if err != nil {
		return model.Proxy{}, ssSubscriptionLineError(line, err.Error())
	}
	rawParams, err := parseSSLineParams(segments[1:])
	if err != nil {
		return model.Proxy{}, ssSubscriptionLineError(line, err.Error())
	}

	return buildSSLineProxy(line, rawParams["tag"], server, port, rawParams["method"], rawParams["password"], rawParams)
}

func parseSurgeSSLine(line string) (model.Proxy, error) {
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return model.Proxy{}, ssSubscriptionLineError(line, "缺少 '=' 分隔符")
	}
	name := strings.TrimSpace(line[:eqIdx])
	if name == "" {
		return model.Proxy{}, ssSubscriptionLineError(line, "节点名称为空")
	}

	segments, err := splitSSSubscriptionSegments(line[eqIdx+1:])
	if err != nil {
		return model.Proxy{}, ssSubscriptionLineError(line, err.Error())
	}
	if len(segments) < 4 {
		return model.Proxy{}, ssSubscriptionLineError(line, "字段数不足，至少需要 type、server、port 和参数")
	}
	if proxyType := strings.ToLower(strings.TrimSpace(segments[0])); proxyType != "ss" && proxyType != "shadowsocks" {
		return model.Proxy{}, ssSubscriptionLineError(line, fmt.Sprintf("type 必须为 ss，当前为 %q", segments[0]))
	}

	port, err := strconv.Atoi(strings.TrimSpace(segments[2]))
	if err != nil {
		return model.Proxy{}, ssSubscriptionLineError(line, fmt.Sprintf("port %q 不是整数", segments[2]))
	}
	if port < 1 || port > 65535 {
		return model.Proxy{}, ssSubscriptionLineError(line, fmt.Sprintf("port %d 超出 1-65535 范围", port))
	}
	server := strings.TrimSpace(segments[1])
	if server == "" {
		return model.Proxy{}, ssSubscriptionLineError(line, "server 为空")
	}

	rawParams, err := parseSSLineParams(segments[3:])
	if err != nil {
		return model.Proxy{}, ssSubscriptionLineError(line, err.Error())
	}

	return buildSSLineProxy(line, name, server, port, rawParams["encrypt-method"], rawParams["password"], rawParams)
}

func buildSSLineProxy(line, name, server string, port int, cipher, password string, rawParams map[string]string) (model.Proxy, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Proxy{}, ssSubscriptionLineError(line, "节点名称为空")
	}
	if cipher == "" {
		return model.Proxy{}, ssSubscriptionLineError(line, "缺少加密方式")
	}
	if password == "" {
		return model.Proxy{}, ssSubscriptionLineError(line, "缺少密码")
	}

	params := map[string]string{
		"cipher":   cipher,
		"password": password,
	}
	if v := rawParams["udp-relay"]; v != "" {
		params["udp-relay"] = v
	}
	if v := rawParams["tfo"]; v != "" {
		params["tfo"] = v
	}
	if v := rawParams["fast-open"]; v != "" {
		params["tfo"] = v
	}

	return model.Proxy{
		Name:   name,
		Type:   "ss",
		Server: server,
		Port:   port,
		Params: params,
		Kind:   model.KindSubscription,
	}, nil
}

func parseSSLineServerPort(hostport string) (string, int, error) {
	server, portStr, err := splitHostPort(strings.TrimSpace(hostport))
	if err != nil {
		return "", 0, err
	}
	if server == "" {
		return "", 0, fmt.Errorf("server 为空")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("port %q 不是整数", portStr)
	}
	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("port %d 超出 1-65535 范围", port)
	}
	return server, port, nil
}

func parseSSLineParams(segments []string) (map[string]string, error) {
	params := make(map[string]string, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		kvIdx := strings.Index(segment, "=")
		if kvIdx < 0 {
			return nil, fmt.Errorf("参数 %q 缺少 '='", segment)
		}
		key := strings.ToLower(strings.TrimSpace(segment[:kvIdx]))
		if key == "" {
			return nil, fmt.Errorf("参数 %q 的 key 为空", segment)
		}
		value, err := unquoteSSLineValue(strings.TrimSpace(segment[kvIdx+1:]))
		if err != nil {
			return nil, fmt.Errorf("参数 %q 的值无效：%v", key, err)
		}
		params[key] = value
	}
	return params, nil
}

func splitSSSubscriptionSegments(s string) ([]string, error) {
	var parts []string
	var b strings.Builder
	var quote rune
	escaped := false

	for _, r := range s {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case quote != 0 && r == '\\':
			b.WriteRune(r)
			escaped = true
		case quote != 0:
			b.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
			b.WriteRune(r)
		case r == ',':
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("存在未闭合的引号")
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	return parts, nil
}

func unquoteSSLineValue(value string) (string, error) {
	if len(value) < 2 {
		return value, nil
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		return strconv.Unquote(value)
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}
	return value, nil
}

func ssSubscriptionLineError(line, reason string) error {
	display := strings.TrimSpace(line)
	if len(display) > 120 {
		display = display[:117] + "..."
	}
	return &errtype.BuildError{
		Code:    errtype.CodeBuildSSURIInvalid,
		Phase:   "source",
		Message: fmt.Sprintf("SS 订阅行 %q 无效：%s", display, reason),
	}
}
