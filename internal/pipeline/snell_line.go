package pipeline

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// errSnellLineSkip is a sentinel signalling that the caller should skip this
// line without treating it as a fatal error (blank lines, comments).
var errSnellLineSkip = errors.New("snell line skipped")

// ParseSnellSurgeLine parses a single Surge-style Snell proxy declaration
// into a model.Proxy. The expected shape is:
//
//	NAME = snell, SERVER, PORT, KEY=VALUE[, KEY=VALUE ...]
//
// Surrounding whitespace around ` = ` and `,` is tolerated. Blank lines and
// lines starting with `#` or `//` return errSnellLineSkip so the caller can
// ignore them without treating them as hard errors.
//
// The returned Proxy carries Kind=KindSnell and Type="snell". All KEY=VALUE
// pairs are copied into Params verbatim (keys kept in their Surge
// lowercase-hyphen form). `psk` is required; all other pairs are optional.
//
// Layering note: Params stores ALL parsed keys, but the Surge renderer only
// emits keys listed in surgeSnellKeyOrder (see internal/render/surge.go);
// unknown keys survive in Params but are not written to Surge output. This
// split lets the parser stay lenient (forward-compatible with new Surge
// options) while the renderer stays deterministic for golden-file testing.
// To support a new Surge option, extend surgeSnellKeyOrder — the parser
// does not need changes.
func ParseSnellSurgeLine(line string) (model.Proxy, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return model.Proxy{}, errSnellLineSkip
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
		return model.Proxy{}, errSnellLineSkip
	}

	// Split NAME from the rest on the first '='.
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx < 0 {
		return model.Proxy{}, snellLineError(line, "缺少 '=' 分隔 name 与 type")
	}
	name := strings.TrimSpace(trimmed[:eqIdx])
	if name == "" {
		return model.Proxy{}, snellLineError(line, "name 为空")
	}
	rest := trimmed[eqIdx+1:]

	// Split remainder by ','. Need at least: type, server, port.
	segments := splitAndTrim(rest, ",")
	if len(segments) < 3 {
		return model.Proxy{}, snellLineError(line, "字段数不足，至少需要 type、server、port")
	}

	if segments[0] != "snell" {
		return model.Proxy{}, snellLineError(line, fmt.Sprintf("type 必须为 snell，当前为 %q", segments[0]))
	}

	server := segments[1]
	if server == "" {
		return model.Proxy{}, snellLineError(line, "server 为空")
	}

	port, err := strconv.Atoi(segments[2])
	if err != nil {
		return model.Proxy{}, snellLineError(line, fmt.Sprintf("port %q 不是整数", segments[2]))
	}
	if port < 1 || port > 65535 {
		return model.Proxy{}, snellLineError(line, fmt.Sprintf("port %d 超出 1-65535 范围", port))
	}

	params := make(map[string]string)
	for _, pair := range segments[3:] {
		if pair == "" {
			continue
		}
		kvIdx := strings.Index(pair, "=")
		if kvIdx < 0 {
			return model.Proxy{}, snellLineError(line, fmt.Sprintf("参数 %q 缺少 '='", pair))
		}
		key := strings.TrimSpace(pair[:kvIdx])
		value := strings.TrimSpace(pair[kvIdx+1:])
		if key == "" {
			return model.Proxy{}, snellLineError(line, fmt.Sprintf("参数 %q 的 key 为空", pair))
		}
		// Duplicate keys: last one wins (matches Surge's permissive behaviour).
		params[key] = value
	}

	if params["psk"] == "" {
		return model.Proxy{}, snellLineError(line, "缺少必填参数 psk")
	}

	return model.Proxy{
		Name:   name,
		Type:   "snell",
		Server: server,
		Port:   port,
		Params: params,
		Kind:   model.KindSnell,
	}, nil
}

// splitAndTrim splits s on sep, trims each part, and returns the slice.
// Empty trailing parts caused by trailing separators are retained as "" so
// callers can distinguish "a,b," from "a,b".
func splitAndTrim(s, sep string) []string {
	raw := strings.Split(s, sep)
	out := make([]string, len(raw))
	for i, p := range raw {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func snellLineError(line, reason string) error {
	display := strings.TrimSpace(line)
	if len(display) > 120 {
		display = display[:117] + "..."
	}
	return &errtype.BuildError{
		Code:    errtype.CodeBuildSnellLineInvalid,
		Phase:   "source",
		Message: fmt.Sprintf("Snell 节点 %q 无效：%s", display, reason),
	}
}
