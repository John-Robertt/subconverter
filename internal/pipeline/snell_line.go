package pipeline

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// errSnellLineSkip 是一个哨兵错误，表示调用方应跳过这一行，
// 而不把它当作致命错误（空行、注释）。
var errSnellLineSkip = errors.New("已跳过 snell 行")

// ParseSnellSurgeLine 把一行 Surge 风格的 Snell 节点声明解析为 model.Proxy。
// 期望的形式是：
//
//	NAME = snell, SERVER, PORT, KEY=VALUE[, KEY=VALUE ...]
//
// ` = ` 和 `,` 两侧的空白可以容忍。空行以及以 `#` 或 `//` 开头的行返回
// errSnellLineSkip，调用方可以忽略它们，而不当作硬错误处理。
//
// 返回的 Proxy 带有 Kind=KindSnell 和 Type="snell"。所有 KEY=VALUE 键值对
// 原样复制到 Params 中（键保持 Surge 的小写连字符形式）。`psk` 必填，
// 其余键值对都是可选的。
//
// 分层说明：Params 保存所有解析出的键，但 Surge 渲染器只输出
// surgeSnellKeyOrder 中列出的键（见 internal/render/surge.go）；
// 未知键保留在 Params 中，但不会写入 Surge 输出。这样拆分后，解析器可以保持宽松
// （向前兼容 Surge 新增的选项），渲染器则保持输出确定，便于 golden 文件测试。
// 要支持新的 Surge 选项，扩展 surgeSnellKeyOrder 即可，解析器无需改动。
func ParseSnellSurgeLine(line string) (model.Proxy, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return model.Proxy{}, errSnellLineSkip
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
		return model.Proxy{}, errSnellLineSkip
	}

	// 在第一个 '=' 处把 NAME 与其余部分分开。
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx < 0 {
		return model.Proxy{}, snellLineError(line, "缺少 '=' 分隔 name 与 type")
	}
	name := strings.TrimSpace(trimmed[:eqIdx])
	if name == "" {
		return model.Proxy{}, snellLineError(line, "name 为空")
	}
	rest := trimmed[eqIdx+1:]

	// 剩余部分按 ',' 拆分。至少需要：type、server、port。
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
		// 键重复时以最后一个为准（与 Surge 的宽松行为一致）。
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

// splitAndTrim 按 sep 拆分 s，去掉每段两侧空白后返回切片。
// 末尾分隔符产生的空段会保留为 ""，调用方因此可以区分 "a,b," 和 "a,b"。
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
