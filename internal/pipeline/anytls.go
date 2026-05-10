package pipeline

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ParseAnyTLSURI parses an AnyTLS URI into a subscription proxy.
//
// Supported form:
//
//	anytls://PASSWORD@SERVER[:PORT][/][?sni=...&insecure=1][#NodeName]
//
// The AnyTLS URI scheme defines auth as the password. Port defaults to 443
// when omitted. Query keys not consumed here are ignored so subscription
// providers can add metadata such as group without breaking parsing.
func ParseAnyTLSURI(raw string) (model.Proxy, error) {
	const prefix = "anytls://"
	if !strings.HasPrefix(raw, prefix) {
		return model.Proxy{}, anyTLSURIError(raw, "缺少 anytls:// 前缀")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return model.Proxy{}, anyTLSURIError(raw, fmt.Sprintf("URL 格式无效：%v", err))
	}
	if u.Scheme != "anytls" {
		return model.Proxy{}, anyTLSURIError(raw, fmt.Sprintf("scheme 必须为 anytls，当前为 %q", u.Scheme))
	}

	name, err := decodeAnyTLSFragmentName(u)
	if err != nil {
		return model.Proxy{}, anyTLSURIError(raw, err.Error())
	}
	if name == "" {
		return model.Proxy{}, anyTLSURIError(raw, "节点名称为空")
	}

	password, err := decodeAnyTLSPassword(u)
	if err != nil {
		return model.Proxy{}, anyTLSURIError(raw, err.Error())
	}
	if password == "" {
		return model.Proxy{}, anyTLSURIError(raw, "缺少 password")
	}

	server := u.Hostname()
	if server == "" {
		return model.Proxy{}, anyTLSURIError(raw, "server 为空")
	}
	port, err := parseAnyTLSPort(u.Port())
	if err != nil {
		return model.Proxy{}, anyTLSURIError(raw, err.Error())
	}

	params := map[string]string{"password": password}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return model.Proxy{}, anyTLSURIError(raw, fmt.Sprintf("query 参数无效：%v", err))
	}
	applyAnyTLSURIQuery(values, params)

	return model.Proxy{
		Name:   name,
		Type:   "anytls",
		Server: server,
		Port:   port,
		Params: params,
		Kind:   model.KindSubscription,
	}, nil
}

func decodeAnyTLSFragmentName(u *url.URL) (string, error) {
	fragment := u.EscapedFragment()
	if fragment == "" {
		fragment = u.Fragment
	}
	if fragment == "" {
		return "", nil
	}
	name, err := url.PathUnescape(fragment)
	if err != nil {
		return "", fmt.Errorf("fragment 编码无效：%v", err)
	}
	return name, nil
}

func decodeAnyTLSPassword(u *url.URL) (string, error) {
	if u.User == nil {
		return "", nil
	}
	password := u.User.Username()
	if password == "" {
		if v, ok := u.User.Password(); ok {
			password = v
		}
	}
	decoded, err := url.PathUnescape(password)
	if err != nil {
		return "", fmt.Errorf("password 编码无效：%v", err)
	}
	return decoded, nil
}

func parseAnyTLSPort(raw string) (int, error) {
	if raw == "" {
		return 443, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("端口 %q 不是整数", raw)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("端口 %d 超出 1-65535 范围", port)
	}
	return port, nil
}

func applyAnyTLSURIQuery(values url.Values, params map[string]string) {
	if v := strings.TrimSpace(values.Get("sni")); v != "" {
		params["sni"] = v
	}
	if anyTLSParamBoolTrue(values.Get("insecure")) {
		params["skip-cert-verify"] = "true"
	}
	if v := strings.TrimSpace(values.Get("fp")); v != "" {
		params["client-fingerprint"] = v
	}
	if v := strings.TrimSpace(values.Get("alpn")); v != "" {
		params["alpn"] = v
	}
}

func parseQuanXAnyTLSLine(line string) (model.Proxy, error) {
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return model.Proxy{}, anyTLSLineError(line, "缺少 '=' 分隔符")
	}

	segments, err := splitSSSubscriptionSegments(line[eqIdx+1:])
	if err != nil {
		return model.Proxy{}, anyTLSLineError(line, err.Error())
	}
	if len(segments) < 2 {
		return model.Proxy{}, anyTLSLineError(line, "字段数不足，至少需要 host:port 和参数")
	}

	server, port, err := parseSSLineServerPort(segments[0])
	if err != nil {
		return model.Proxy{}, anyTLSLineError(line, err.Error())
	}
	rawParams, err := parseSSLineParams(segments[1:])
	if err != nil {
		return model.Proxy{}, anyTLSLineError(line, err.Error())
	}

	return buildAnyTLSLineProxy(line, rawParams["tag"], server, port, rawParams)
}

func parseSurgeAnyTLSLine(line string) (model.Proxy, error) {
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return model.Proxy{}, anyTLSLineError(line, "缺少 '=' 分隔符")
	}
	name := strings.TrimSpace(line[:eqIdx])
	if name == "" {
		return model.Proxy{}, anyTLSLineError(line, "节点名称为空")
	}

	segments, err := splitSSSubscriptionSegments(line[eqIdx+1:])
	if err != nil {
		return model.Proxy{}, anyTLSLineError(line, err.Error())
	}
	if len(segments) < 4 {
		return model.Proxy{}, anyTLSLineError(line, "字段数不足，至少需要 type、server、port 和参数")
	}
	if proxyType := strings.ToLower(strings.TrimSpace(segments[0])); proxyType != "anytls" {
		return model.Proxy{}, anyTLSLineError(line, fmt.Sprintf("type 必须为 anytls，当前为 %q", segments[0]))
	}

	port, err := strconv.Atoi(strings.TrimSpace(segments[2]))
	if err != nil {
		return model.Proxy{}, anyTLSLineError(line, fmt.Sprintf("port %q 不是整数", segments[2]))
	}
	if port < 1 || port > 65535 {
		return model.Proxy{}, anyTLSLineError(line, fmt.Sprintf("port %d 超出 1-65535 范围", port))
	}
	server := strings.TrimSpace(segments[1])
	if server == "" {
		return model.Proxy{}, anyTLSLineError(line, "server 为空")
	}

	rawParams, err := parseSSLineParams(segments[3:])
	if err != nil {
		return model.Proxy{}, anyTLSLineError(line, err.Error())
	}

	return buildAnyTLSLineProxy(line, name, server, port, rawParams)
}

func buildAnyTLSLineProxy(line, name, server string, port int, rawParams map[string]string) (model.Proxy, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Proxy{}, anyTLSLineError(line, "节点名称为空")
	}
	password := rawParams["password"]
	if password == "" {
		return model.Proxy{}, anyTLSLineError(line, "缺少 password")
	}

	params := map[string]string{"password": password}
	copyAnyTLSParam(params, rawParams, "sni", "sni")
	copyAnyTLSParam(params, rawParams, "tls-host", "sni")
	copyAnyTLSParam(params, rawParams, "skip-cert-verify", "skip-cert-verify")
	if v := strings.TrimSpace(rawParams["tls-verification"]); v != "" {
		params["skip-cert-verify"] = boolString(!anyTLSParamBoolTrue(v))
	}
	copyAnyTLSParam(params, rawParams, "reuse", "reuse")
	copyAnyTLSParam(params, rawParams, "udp-relay", "udp-relay")
	copyAnyTLSParam(params, rawParams, "fast-open", "tfo")
	copyAnyTLSParam(params, rawParams, "tfo", "tfo")
	copyAnyTLSParam(params, rawParams, "client-fingerprint", "client-fingerprint")
	copyAnyTLSParam(params, rawParams, "alpn", "alpn")
	copyAnyTLSParam(params, rawParams, "idle-session-check-interval", "idle-session-check-interval")
	copyAnyTLSParam(params, rawParams, "idle-session-timeout", "idle-session-timeout")
	copyAnyTLSParam(params, rawParams, "min-idle-session", "min-idle-session")
	copyAnyTLSParam(params, rawParams, "server-cert-fingerprint-sha256", "server-cert-fingerprint-sha256")

	return model.Proxy{
		Name:   name,
		Type:   "anytls",
		Server: server,
		Port:   port,
		Params: params,
		Kind:   model.KindSubscription,
	}, nil
}

func copyAnyTLSParam(dst, src map[string]string, srcKey, dstKey string) {
	if v := strings.TrimSpace(src[srcKey]); v != "" {
		dst[dstKey] = v
	}
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func anyTLSParamBoolTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func isQuanXAnyTLSLine(line string) bool {
	eqIdx := strings.Index(line, "=")
	return eqIdx > 0 && strings.EqualFold(strings.TrimSpace(line[:eqIdx]), "anytls")
}

func isSurgeAnyTLSLine(line string) bool {
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return false
	}
	segments, err := splitSSSubscriptionSegments(line[eqIdx+1:])
	if err != nil || len(segments) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(segments[0]), "anytls")
}

func anyTLSURIError(uri, reason string) error {
	display := strings.TrimSpace(uri)
	if len(display) > 120 {
		display = display[:117] + "..."
	}
	return &errtype.BuildError{
		Code:    errtype.CodeBuildAnyTLSURIInvalid,
		Phase:   "source",
		Message: fmt.Sprintf("AnyTLS URI %q 无效：%s", display, reason),
	}
}

func anyTLSLineError(line, reason string) error {
	display := strings.TrimSpace(line)
	if len(display) > 120 {
		display = display[:117] + "..."
	}
	return &errtype.BuildError{
		Code:    errtype.CodeBuildAnyTLSLineInvalid,
		Phase:   "source",
		Message: fmt.Sprintf("AnyTLS 订阅行 %q 无效：%s", display, reason),
	}
}
