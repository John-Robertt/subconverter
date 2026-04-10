package pipeline

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
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
		return model.Proxy{}, ssError(raw, "missing ss:// prefix")
	}

	body := raw[len(prefix):]

	// Split fragment (node name).
	name := ""
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		fragment := body[idx+1:]
		body = body[:idx]
		decoded, err := url.PathUnescape(fragment)
		if err != nil {
			return model.Proxy{}, ssError(raw, fmt.Sprintf("invalid fragment encoding: %v", err))
		}
		name = decoded
	}
	if name == "" {
		return model.Proxy{}, ssError(raw, "missing or empty node name")
	}

	query := ""
	if idx := strings.Index(body, "?"); idx >= 0 {
		query = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimSuffix(body, "/")

	// Split userinfo and host:port on the last '@'.
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return model.Proxy{}, ssError(raw, "missing @ separator")
	}
	userinfo := body[:atIdx]
	hostport := body[atIdx+1:]

	method, password, err := parseSSUserinfo(userinfo)
	if err != nil {
		return model.Proxy{}, ssError(raw, err.Error())
	}
	if method == "" {
		return model.Proxy{}, ssError(raw, "empty cipher method")
	}

	server, portStr, err := parseSSHostPort(hostport)
	if err != nil {
		return model.Proxy{}, ssError(raw, err.Error())
	}
	if server == "" {
		return model.Proxy{}, ssError(raw, "empty server host")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return model.Proxy{}, ssError(raw, fmt.Sprintf("invalid port %q", portStr))
	}
	if port < 1 || port > 65535 {
		return model.Proxy{}, ssError(raw, fmt.Sprintf("port %d out of range 1-65535", port))
	}

	params := map[string]string{
		"cipher":   method,
		"password": password,
	}

	plugin, err := parseSSQuery(query)
	if err != nil {
		return model.Proxy{}, ssError(raw, err.Error())
	}

	return model.Proxy{
		Name:   name,
		Type:   "ss",
		Server: server,
		Port:   port,
		Params: params,
		Plugin: plugin,
		Kind:   model.KindSubscription,
	}, nil
}

func parseSSUserinfo(userinfo string) (string, string, error) {
	if plain, err := url.PathUnescape(userinfo); err == nil {
		parts := strings.SplitN(plain, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	decoded, err := decodeBase64(userinfo)
	if err != nil {
		return "", "", fmt.Errorf("invalid userinfo: %v", err)
	}

	parts := strings.SplitN(decoded, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("userinfo missing ':' separator between method and password")
	}

	return parts[0], parts[1], nil
}

func parseSSHostPort(hostport string) (string, string, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return "", "", fmt.Errorf("missing port")
		}
		return "", "", fmt.Errorf("invalid host:port %q", hostport)
	}
	return host, port, nil
}

func parseSSQuery(rawQuery string) (*model.Plugin, error) {
	if rawQuery == "" {
		return nil, nil
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("invalid query string: %v", err)
	}

	if pluginSpec := strings.TrimSpace(values.Get("plugin")); pluginSpec != "" {
		return parseSSPlugin(pluginSpec)
	}

	return nil, nil
}

func parseSSPlugin(spec string) (*model.Plugin, error) {
	parts, err := splitEscaped(spec, ';')
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return nil, fmt.Errorf("empty plugin name")
	}

	plugin := &model.Plugin{Name: strings.TrimSpace(parts[0])}

	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		key, value, hasValue, err := splitEscapedKeyValue(part)
		if err != nil {
			return nil, err
		}
		if key == "" {
			return nil, fmt.Errorf("invalid plugin option %q", part)
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
		return nil, fmt.Errorf("unterminated escape sequence in plugin options")
	}

	parts = append(parts, b.String())
	return parts, nil
}

func splitEscapedKeyValue(s string) (string, string, bool, error) {
	var key strings.Builder
	var value strings.Builder
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
		return "", "", false, fmt.Errorf("unterminated escape sequence in plugin options")
	}

	return strings.TrimSpace(key.String()), strings.TrimSpace(value.String()), seenEquals, nil
}

// decodeBase64 tries multiple base64 encodings to handle both
// padded and unpadded variants commonly found in SS subscriptions.
func decodeBase64(s string) (string, error) {
	// Try standard encoding with padding first.
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	// Try without padding (common in SS URIs).
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	// Try URL-safe variants.
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	return "", fmt.Errorf("not valid base64")
}

func ssError(uri, reason string) error {
	// Truncate URI in error messages to avoid leaking full credentials.
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
