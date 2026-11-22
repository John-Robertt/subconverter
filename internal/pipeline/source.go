package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/proxyparse"
)

// defaultFetchOrder 是 Sources.FetchOrder 为空时使用的后备遍历顺序
// （测试中跳过 YAML 反序列化、直接在内存中构造的 Config 实例会出现这种情况）。
// 从 YAML 加载的配置则按声明顺序填充 FetchOrder。
var defaultFetchOrder = []string{"subscriptions", "snell", "vless"}

// ChainTemplate 是带 relay_through 的自定义节点的运行时形式。它由 Source 产出，
// 再由 Group 展开为链式节点。
type ChainTemplate struct {
	Name         string
	Type         string
	Server       string
	Port         int
	Params       map[string]string
	Plugin       *model.Plugin
	RelayThrough *config.PreparedRelayThrough
}

// SourceResult 是 Source 阶段的输出。Proxies 包含全部原始节点
// （拉取类节点 + 独立的自定义节点）；ChainTemplates 携带供 Group 阶段使用的
// relay_through 声明。NextProxyID 是 Source 尚未分配的第一个编号，
// 过滤之后仍然保留，这样 Group 永远不会复用某个已被排除的原始节点的编号。
type SourceResult struct {
	Proxies        []model.Proxy
	ChainTemplates []ChainTemplate
	NextProxyID    model.ProxyID
}

// sourcePrepared 针对启动期准备好的来源声明，执行请求时的 Source 阶段。
//
// 三类拉取来源之间的遍历顺序遵循 sources.FetchOrder，它反映 YAML 中的声明顺序
// （例如先写 `snell:`，再写 `vless:`，最后写 `subscriptions:`，输出中的节点
// 就恰好按这个顺序排列）。FetchOrder 为空时（单元测试在内存中构造的 Config），
// 使用一个确定的默认顺序。
//
// 来源按顺序逐个处理，以保证 ProxyID 的分配是确定的。拉取错误会立即失败；
// 订阅中个别格式错误的 SS/AnyTLS 行会被跳过，但一个订阅最终没有产生任何
// 有效节点时报错。VLESS 和 Snell 来源只要有一行解析失败就中止
// （它们是手工维护的小列表，严格失败能尽早暴露笔误）。
func sourcePrepared(ctx context.Context, sources config.PreparedSources, fetcher fetch.Fetcher) (*SourceResult, error) {
	var fetchedProxies []model.Proxy

	order := sources.FetchOrder
	if len(order) == 0 {
		order = defaultFetchOrder
	}

	for _, kind := range order {
		switch kind {
		case "subscriptions":
			for index, sub := range sources.Subscriptions {
				proxies, err := loadSubscription(ctx, fetcher, sub)
				if err != nil {
					if sub.Content != "" {
						var sourceErr *errtype.FetchError
						if errors.As(err, &sourceErr) {
							located := *sourceErr
							located.Message = fmt.Sprintf("sources.subscriptions[%d].content：%s", index, sourceErr.Message)
							return nil, &located
						}
					}
					return nil, err
				}
				fetchedProxies = append(fetchedProxies, proxies...)
			}
		case "snell", "vless":
			entries := sources.Snell
			if kind == "vless" {
				entries = sources.VLess
			}
			for index, source := range entries {
				location, body, err := loadSourceBody(ctx, fetcher, source, fmt.Sprintf("sources.%s[%d].content", kind, index))
				if err != nil {
					return nil, err
				}
				proxies, err := parseNodePool(kind, location, body)
				if err != nil {
					return nil, err
				}
				fetchedProxies = append(fetchedProxies, proxies...)
			}
		default:
			// FetchOrder 只应包含 sourceFetchKeys 中的键
			// （由 Sources.UnmarshalYAML 保证）。走到这个分支，
			// 说明某个内存中的 Config 直接设置了 FetchOrder，
			// 且其中有笔误或未知类型；这里把它报出来，而不是悄悄丢掉这个来源。
			return nil, &errtype.BuildError{
				Code:    errtype.CodeBuildValidationFailed,
				Phase:   "source",
				Message: fmt.Sprintf("Sources.FetchOrder 中有未知的拉取类型 %q（请检查 Sources.UnmarshalYAML 和 defaultFetchOrder）", kind),
			}
		}
	}

	customProxies, chainTemplates := convertCustomProxies(sources.CustomProxies)

	proxies := make([]model.Proxy, 0, len(fetchedProxies)+len(customProxies))
	proxies = append(proxies, fetchedProxies...)
	proxies = append(proxies, customProxies...)
	proxies = assignProxyIDs(proxies)
	if err := validateGeneratedProxies("source", proxies); err != nil {
		return nil, err
	}
	return &SourceResult{
		Proxies:        proxies,
		ChainTemplates: chainTemplates,
		NextProxyID:    model.ProxyID(len(proxies) + 1),
	}, nil
}

func assignProxyIDs(proxies []model.Proxy) []model.Proxy {
	result := make([]model.Proxy, len(proxies))
	for index, proxy := range proxies {
		proxy.ID = model.ProxyID(index + 1)
		result[index] = proxy
	}
	return result
}

// loadSubscription 读取内联/本地内容，或拉取订阅 URL。结构化的 Clash YAML 和
// Surge CONF / Loon 文档会先于传统的 Base64/明文订阅列表格式被识别。
func loadSubscription(ctx context.Context, fetcher fetch.Fetcher, sub config.Source) ([]model.Proxy, error) {
	rawURL, body, err := loadSourceBody(ctx, fetcher, sub, "inline:subscription")
	if err != nil {
		return nil, err
	}
	proxies, _, err := parseSubscriptionBody(rawURL, body, sub.Format, sub.Content != "" || sub.ContentFile != "")
	return proxies, err
}

// loadSourceBody 在准备好的输入中恰好选用一种；内联来源和本地来源从不使用远程 Fetcher。
func loadSourceBody(ctx context.Context, fetcher fetch.Fetcher, source config.Source, inlineLocation string) (string, []byte, error) {
	if source.ContentFile != "" {
		body, err := fetch.LoadResource(ctx, source.ContentFile, fetcher)
		return source.ContentFile, body, err
	}
	if source.Content != "" {
		return inlineLocation, []byte(source.Content), nil
	}
	body, err := fetcher.Fetch(ctx, source.URL)
	return source.URL, body, err
}

// parseNodePool 对 URL、本地和编辑后的内容使用同一个严格的协议解析器。
func parseNodePool(kind, location string, body []byte) ([]model.Proxy, error) {
	switch kind {
	case "snell":
		return parseSnellSource(location, body)
	case "vless":
		return parseVLessSource(location, body)
	default:
		return nil, &errtype.BuildError{Code: errtype.CodeBuildValidationFailed, Phase: "source", Message: "节点池类型无效：" + kind}
	}
}

func parseSubscriptionBody(rawURL string, body []byte, format string, allowFragment bool) ([]model.Proxy, string, error) {
	// 粘贴的 INI 节点声明不需要文档外壳，保存后的副本也一样。
	if allowFragment && (format == "loon" || format == "surge") && !hasINISection(string(body)) {
		body = append([]byte("[Proxy]\n"), body...)
	}

	document, err := parseSubscriptionDocument(rawURL, body, format)
	if document != nil {
		if err != nil {
			return nil, "", &errtype.FetchError{
				Code:    errtype.CodeFetchSubscriptionFormatInvalid,
				URL:     fetch.SanitizeURL(rawURL),
				Message: subscriptionDocumentFormatLabel(document.Format) + " 订阅格式无效：" + err.Error(),
				Cause:   err,
			}
		}
		if len(document.Proxies) == 0 {
			return nil, "", &errtype.FetchError{
				Code:    errtype.CodeFetchSubscriptionEmpty,
				URL:     fetch.SanitizeURL(rawURL),
				Message: subscriptionDocumentFormatLabel(document.Format) + " 订阅未产生任何有效节点",
			}
		}
		return document.Proxies, string(document.Format), nil
	}

	text, err := subscriptionBodyText(body)
	if err != nil {
		return nil, "", &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionBase64Invalid,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "订阅内容不是合法的 Clash YAML、Surge CONF、Loon CONF、Base64，也不是明文 SS/AnyTLS 订阅列表",
			Cause:   err,
		}
	}

	lines := splitSubscriptionLines(text)
	var proxies []model.Proxy
	for _, line := range lines {
		proxy, err := ParseSubscriptionLine(line)
		if err != nil {
			// 跳过个别格式错误的订阅行。
			continue
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, "", &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "订阅未产生任何有效节点",
		}
	}

	return proxies, "text", nil
}

func subscriptionDocumentFormatLabel(format model.NativeProxyFormat) string {
	switch format {
	case model.NativeProxyFormatClash:
		return "Clash YAML"
	case model.NativeProxyFormatSurge:
		return "Surge CONF"
	case model.NativeProxyFormatLoon:
		return "Loon CONF"
	default:
		return "结构化"
	}
}

// parseSnellSource 每行解析一条 Surge 风格的 Snell 声明。
// 空行和注释行被跳过；其他任何无效行都会中止该来源，并保留它的物理行号。
func parseSnellSource(location string, body []byte) ([]model.Proxy, error) {
	var proxies []model.Proxy
	sanitizedLocation := fetch.SanitizeURL(location)
	for lineNo, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		proxy, parseErr := ParseSnellSurgeLine(line)
		if parseErr != nil {
			if errors.Is(parseErr, errSnellLineSkip) {
				continue
			}
			return nil, wrapSnellSourceParseError(sanitizedLocation, lineNo+1, parseErr)
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(location),
			Message: "Snell 来源未产生任何有效节点",
		}
	}

	return proxies, nil
}

func wrapSnellSourceParseError(sanitizedLocation string, lineNo int, parseErr error) error {
	detail := parseErr.Error()
	var buildErr *errtype.BuildError
	if errors.As(parseErr, &buildErr) {
		detail = buildErr.Message
	}

	return &errtype.BuildError{
		Code:    errtype.CodeBuildSnellLineInvalid,
		Phase:   "source",
		Message: fmt.Sprintf(`Snell 来源 %q 第 %d 行解析失败：%s`, sanitizedLocation, lineNo, detail),
		Cause:   parseErr,
	}
}

// parseVLessSource 每行解析一条 VLESS URI，失败策略与 Snell 一样严格。
func parseVLessSource(location string, body []byte) ([]model.Proxy, error) {
	var proxies []model.Proxy
	sanitizedLocation := fetch.SanitizeURL(location)
	for lineNo, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		proxy, parseErr := ParseVLessURI(line)
		if parseErr != nil {
			return nil, wrapVLessSourceParseError(sanitizedLocation, lineNo+1, parseErr)
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(location),
			Message: "VLESS 来源未产生任何有效节点",
		}
	}

	return proxies, nil
}

func wrapVLessSourceParseError(sanitizedLocation string, lineNo int, parseErr error) error {
	detail := parseErr.Error()
	var buildErr *errtype.BuildError
	if errors.As(parseErr, &buildErr) {
		detail = buildErr.Message
	}

	return &errtype.BuildError{
		Code:    errtype.CodeBuildVLessSourceLineInvalid,
		Phase:   "source",
		Message: fmt.Sprintf(`VLESS 来源 %q 第 %d 行解析失败：%s`, sanitizedLocation, lineNo, detail),
		Cause:   parseErr,
	}
}

// subscriptionBodyText 同时接受常规的 base64 订阅，以及采用所支持的
// SS/AnyTLS 行格式的明文列表。
func subscriptionBodyText(body []byte) (string, error) {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "", nil
	}
	decoded, err := decodeBase64(s)
	if err == nil {
		return decoded, nil
	}
	if looksLikePlainSubscription(s) {
		return s, nil
	}
	return "", err
}

func looksLikePlainSubscription(text string) bool {
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if isPlainSubscriptionLine(line) {
			return true
		}
	}
	return false
}

// splitSubscriptionLines 去掉每行两侧空白，并跳过空行和注释行。
func splitSubscriptionLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// convertCustomProxies 把启动期准备好的自定义节点转换为独立的自定义节点，
// 或转换为链式模板，留待 Group 阶段展开。
func convertCustomProxies(cps []config.PreparedCustomProxy) ([]model.Proxy, []ChainTemplate) {
	proxies := make([]model.Proxy, 0, len(cps))
	chainTemplates := make([]ChainTemplate, 0, len(cps))
	for _, cp := range cps {
		if cp.RelayThrough != nil {
			chainTemplates = append(chainTemplates, ChainTemplate{
				Name:         cp.Name,
				Type:         cp.Parsed.Type,
				Server:       cp.Parsed.Server,
				Port:         cp.Parsed.Port,
				Params:       copyParams(cp.Parsed.Params),
				Plugin:       convertPluginSpec(cp.Parsed.Plugin),
				RelayThrough: cp.RelayThrough,
			})
			continue
		}
		proxies = append(proxies, model.Proxy{
			Name:   cp.Name,
			Type:   cp.Parsed.Type,
			Server: cp.Parsed.Server,
			Port:   cp.Parsed.Port,
			Params: copyParams(cp.Parsed.Params),
			Plugin: convertPluginSpec(cp.Parsed.Plugin),
			Kind:   model.KindCustom,
		})
	}
	return proxies, chainTemplates
}

// copyParams 返回 Params map 的浅拷贝，避免多个节点共享同一个 map。
// Source 阶段和 Group 阶段都会用到它。
func copyParams(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyPlugin(src *model.Plugin) *model.Plugin {
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

func convertPluginSpec(src *proxyparse.PluginSpec) *model.Plugin {
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

func hasINISection(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if isINISectionHeader(strings.TrimPrefix(strings.TrimSpace(line), "\ufeff")) {
			return true
		}
	}
	return false
}
