package ssparse

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/model"
)

// Result holds the parsed components of an SS URI body.
type Result struct {
	Name     string // from fragment; empty if fragment absent or stripped
	Server   string
	Port     int
	Cipher   string
	Password string
	Plugin   *model.Plugin
}

// ParseBody parses the body of an SS URI (everything after "ss://").
//
// If keepFragment is true the fragment is decoded and stored in Result.Name;
// if false the fragment is silently stripped. Either way, parsing proceeds
// identically for the remaining components.
func ParseBody(body string, keepFragment bool) (Result, error) {
	var name string
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		if keepFragment {
			decoded, err := url.PathUnescape(body[idx+1:])
			if err != nil {
				return Result{}, fmt.Errorf("fragment 编码无效：%v", err)
			}
			name = decoded
		}
		body = body[:idx]
	}

	var rawQuery string
	if idx := strings.Index(body, "?"); idx >= 0 {
		rawQuery = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimSuffix(body, "/")

	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return Result{}, fmt.Errorf("缺少 @ 分隔符")
	}
	userinfo := body[:atIdx]
	hostport := body[atIdx+1:]

	method, password, err := decodeUserinfo(userinfo)
	if err != nil {
		return Result{}, err
	}
	if method == "" {
		return Result{}, fmt.Errorf("加密方式为空")
	}

	server, portStr, err := splitHostPort(hostport)
	if err != nil {
		return Result{}, err
	}
	if server == "" {
		return Result{}, fmt.Errorf("缺少服务器地址")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Result{}, fmt.Errorf("端口 %q 不是合法数字", portStr)
	}
	if port < 1 || port > 65535 {
		return Result{}, fmt.Errorf("端口 %d 超出 1-65535 范围", port)
	}

	plugin, err := parseQueryPlugin(rawQuery)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Name:     name,
		Server:   server,
		Port:     port,
		Cipher:   method,
		Password: password,
		Plugin:   plugin,
	}, nil
}

// decodeUserinfo extracts method and password from SS userinfo.
// Tries plain-text (percent-decoded) first, then base64.
// This order is safe: base64 characters (A-Z a-z 0-9 + / =) never
// contain ':', so a plain-text hit with ':' is unambiguously literal.
func decodeUserinfo(userinfo string) (string, string, error) {
	if plain, err := url.PathUnescape(userinfo); err == nil {
		parts := strings.SplitN(plain, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	decoded, err := decodeBase64Flexible(userinfo)
	if err != nil {
		return "", "", fmt.Errorf("userinfo 无法解码：%v", err)
	}

	parts := strings.SplitN(decoded, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("userinfo 缺少 method:password 分隔符")
	}

	return parts[0], parts[1], nil
}

func splitHostPort(hostport string) (string, string, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return "", "", fmt.Errorf("缺少端口")
		}
		return "", "", fmt.Errorf("host:port 格式无效：%q", hostport)
	}
	return host, port, nil
}

// decodeBase64Flexible tries multiple base64 encodings to handle both
// padded and unpadded variants commonly found in SS URIs.
func decodeBase64Flexible(s string) (string, error) {
	for _, enc := range []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	} {
		if b, err := enc(s); err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("不是合法的 base64")
}

func parseQueryPlugin(rawQuery string) (*model.Plugin, error) {
	if rawQuery == "" {
		return nil, nil
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("query 格式无效：%v", err)
	}

	pluginSpec := strings.TrimSpace(values.Get("plugin"))
	if pluginSpec == "" {
		return nil, nil
	}

	return parsePluginSpec(pluginSpec)
}

func parsePluginSpec(spec string) (*model.Plugin, error) {
	parts, err := splitEscaped(spec, ';')
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return nil, fmt.Errorf("plugin 名称为空")
	}

	plugin := &model.Plugin{Name: strings.TrimSpace(parts[0])}

	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		key, value, hasValue, err := splitEscapedKV(part)
		if err != nil {
			return nil, err
		}
		if key == "" {
			return nil, fmt.Errorf("plugin 选项 %q 无效", part)
		}
		if plugin.Opts == nil {
			plugin.Opts = make(map[string]string)
		}
		if hasValue {
			plugin.Opts[key] = value
		} else {
			plugin.Opts[key] = ""
		}
	}

	return plugin, nil
}

func splitEscaped(s string, sep rune) ([]string, error) {
	var parts []string
	var b strings.Builder
	escaped := false

	for _, r := range s {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == sep:
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}

	if escaped {
		return nil, fmt.Errorf("plugin 选项中存在未终止的转义序列")
	}

	parts = append(parts, b.String())
	return parts, nil
}

func splitEscapedKV(s string) (string, string, bool, error) {
	var key, value strings.Builder
	escaped := false
	seenEquals := false

	for _, r := range s {
		switch {
		case escaped:
			if seenEquals {
				value.WriteRune(r)
			} else {
				key.WriteRune(r)
			}
			escaped = false
		case r == '\\':
			escaped = true
		case r == '=' && !seenEquals:
			seenEquals = true
		default:
			if seenEquals {
				value.WriteRune(r)
			} else {
				key.WriteRune(r)
			}
		}
	}

	if escaped {
		return "", "", false, fmt.Errorf("plugin 选项中存在未终止的转义序列")
	}

	return strings.TrimSpace(key.String()), strings.TrimSpace(value.String()), seenEquals, nil
}
