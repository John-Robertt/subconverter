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

// uuidRe matches a standard dash-delimited UUID (case-insensitive).
// VLESS has no base64 userinfo convention (unlike SS), so the parser only
// accepts this canonical form. Non-standard 32-char "pseudo-UUID" values
// that some V2Ray forks accept are rejected here to keep parser strict.
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// vlessKnownQueryKeys maps transport-independent URI query keys to Params
// keys. Keys are stored under Clash target names so the renderer reads them
// verbatim without rename indirection (domain-model.md convention).
//
// Note `type` is NOT in this map: it is handled separately as a transport
// normalization + dispatch key (see applyVLessQuery). `path` / `host` / `serviceName` /
// `mode` are also not here — they are transport-specific and dispatched by
// applyVLessTransportQuery into network-specific Params keys.
//
// Keys NOT in this map and not handled above are silently ignored (lenient,
// matches ssuri.go).
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

// vlessAllowedNetworks enumerates the transport values that receive dedicated
// Clash transport-opts handling in this codebase. Values outside this set are
// normalized to tcp (see normalizeVLessNetwork) to match Mihomo's documented
// fallback behavior.
var vlessAllowedNetworks = map[string]bool{
	"tcp":   true,
	"ws":    true,
	"http":  true,
	"h2":    true,
	"grpc":  true,
	"xhttp": true,
}

// ParseVLessURI parses a standard VLESS URI into a model.Proxy.
//
// Supported form: vless://UUID@server:port[/][?query][#NodeName]
//
// Design choices embedded here:
//   - `security` accepts only "none" / "tls" / "reality"; other values error.
//     Rationale: these are the VLESS spec's three modes; any other value
//     would mean a Clash client that can't dial the node.
//   - `encryption` is copied through when non-empty. Rationale: Mihomo now
//     supports non-"none" VLESS encryption modes, so rejecting them would
//     make otherwise valid URIs unusable.
//   - `type` (transport) is normalized into Params["network"] using Mihomo's
//     fallback semantics: known values {tcp,ws,http,h2,grpc,xhttp} are kept,
//     while missing/unknown values become tcp.
//   - Empty-value query keys are not stored. Rationale: match ssuri.go's
//     treatment of empty plugin values; reduces churn in Params.
//   - Unknown query keys are silently ignored. Rationale: forward-compat with
//     future V2Ray/Xray query extensions.
//
// Returned Proxy: Kind=KindVLess, Type="vless".
func ParseVLessURI(raw string) (model.Proxy, error) {
	const prefix = "vless://"
	if !strings.HasPrefix(raw, prefix) {
		return model.Proxy{}, vlessURIError(raw, "缺少 vless:// 前缀")
	}

	body := raw[len(prefix):]

	// Fragment → Name. Required, non-empty.
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

	// Query.
	query := ""
	if idx := strings.Index(body, "?"); idx >= 0 {
		query = body[idx+1:]
		body = body[:idx]
	}
	body = strings.TrimSuffix(body, "/")

	// Userinfo (UUID) / hostport split on LAST '@' — a fragment might contain '@'
	// but we stripped that already; this handles unusual encoded passwords
	// in uncommon fork dialects.
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
		"uuid": userinfo,
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

// applyVLessQuery validates and copies known query values into params.
//
// Step 1: determine `network` by reading the `type` query. Missing or unknown
// values normalize to tcp, matching Mihomo's documented fallback behavior.
// Step 2: copy transport-independent keys via vlessKnownQueryKeys.
// Step 3: dispatch transport-specific keys (path/host/serviceName/mode) into
// network-specific Params keys so the renderer emits the right *-opts block
// without runtime dispatch.
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

// normalizeVLessNetwork maps URI query `type` values onto the transport names
// we persist in Params["network"]. Mihomo documents tcp as the default when
// `type` is omitted or set to any unknown value, so this helper mirrors that
// client-facing contract instead of failing strict.
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

// applyVLessTransportQuery dispatches transport-layer query keys into
// transport-specific Params entries based on network. The key naming maps
// 1:1 to the Clash *-opts fields (ws-opts.path, grpc-opts.grpc-service-name,
// etc.), so the renderer looks up `<network>-<field>` directly.
//
// URI query keys handled per network:
//   - ws:    path → ws-path; host → ws-host (becomes ws-opts.headers.Host)
//   - http:  path → http-path; host → http-host (becomes http-opts.headers.Host list)
//   - h2:    path → h2-path; host → h2-host (becomes h2-opts.host list)
//   - grpc:  serviceName → grpc-service-name
//   - xhttp: path → xhttp-path; host → xhttp-host; mode → xhttp-mode
//   - tcp:   (no transport-specific keys)
//
// Values are trimmed; empty values are not stored. The renderer treats a
// missing transport key as "emit the opts block without that sub-field".
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

// vlessURIError produces a BuildError with the URI truncated for display,
// matching the ssError truncation convention in ssuri.go.
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
