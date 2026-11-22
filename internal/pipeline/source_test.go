package pipeline

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/proxyparse"
)

// --- 测试辅助函数 ---

type fakeFetcher struct {
	responses map[string][]byte
	err       error
}

func (f *fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	if f.err != nil {
		return nil, &errtype.FetchError{Code: errtype.CodeFetchRequestFailed, URL: url, Message: "请求上游失败：" + f.err.Error(), Cause: f.err}
	}
	body, ok := f.responses[url]
	if !ok {
		return nil, &errtype.FetchError{Code: errtype.CodeFetchStatusInvalid, URL: url, Message: "上游返回 HTTP 404"}
	}
	return body, nil
}

func makeSubResponse(uris ...string) []byte {
	joined := strings.Join(uris, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(joined)))
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", path, err)
	}
	return data
}

func baseCfg() *config.Config {
	return &config.Config{
		Sources: config.Sources{},
	}
}

func assertSequentialProxyIDs(t *testing.T, proxies []model.Proxy) {
	t.Helper()
	for index, proxy := range proxies {
		want := model.ProxyID(index + 1)
		if proxy.ID != want {
			t.Errorf("proxy[%d].ID = %d，期望 %d", index, proxy.ID, want)
		}
	}
}

// --- Source 测试 ---

func TestSource_SingleSubscription(t *testing.T) {
	body := mustReadFile(t, "../../testdata/subscriptions/sample.txt")
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub1.example.com/api?token=secret"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub1.example.com/api?token=secret": body,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 5 {
		t.Fatalf("得到 %d 个节点，期望 5 个", len(proxies))
	}

	// 确认节点顺序与订阅中的顺序一致。
	wantNames := []string{"HK-01", "HK-02", "SG-01", "JP-东京-01", "US-洛杉矶-01"}
	for i, want := range wantNames {
		if proxies[i].Name != want {
			t.Errorf("proxy[%d].Name = %q，期望 %q", i, proxies[i].Name, want)
		}
		if proxies[i].Kind != model.KindSubscription {
			t.Errorf("proxy[%d].Kind = %q，期望 %q", i, proxies[i].Kind, model.KindSubscription)
		}
	}
}

// T-SRC-ANYTLS-005：订阅内容可以是 base64 编码的 AnyTLS URI 列表。
func TestSource_SubscriptionAnyTLSBase64(t *testing.T) {
	body := makeSubResponse(
		"anytls://1b3614e427e3451b@demo.com:3383/?peer=cache-proxy.example.com&insecure=1#HK-AnyTLS-01",
		"anytls://1b3614e427e3451b@demo.com:3814/?peer=cache-proxy.example.com&insecure=1#HK-AnyTLS-02",
	)
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{{URL: "https://sub.example.com/anytls"}}

	f := &fakeFetcher{responses: map[string][]byte{"https://sub.example.com/anytls": body}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if len(source.Proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(source.Proxies))
	}
	for _, proxy := range source.Proxies {
		if proxy.Kind != model.KindSubscription || proxy.Type != "anytls" {
			t.Errorf("节点 %q 的 kind/type = %q/%q，期望 subscription/anytls", proxy.Name, proxy.Kind, proxy.Type)
		}
		if proxy.Params["password"] != "1b3614e427e3451b" {
			t.Errorf("节点 %q 的 password = %q，期望 fixture 中的密码", proxy.Name, proxy.Params["password"])
		}
		if proxy.Params["sni"] != "cache-proxy.example.com" {
			t.Errorf("节点 %q 的 sni = %q，期望由 peer 查询参数映射为 sni", proxy.Name, proxy.Params["sni"])
		}
	}
}

// T-SRC-ANYTLS-006：订阅内容可以是明文 AnyTLS 列表。
func TestSource_SubscriptionAnyTLSPlainText(t *testing.T) {
	body := []byte("anytls=demo.com:3383, password=secret, tls-host=edge.example.com, tls-verification=false, tag=HK-AnyTLS\n")
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{{URL: "https://sub.example.com/plain-anytls"}}

	f := &fakeFetcher{responses: map[string][]byte{"https://sub.example.com/plain-anytls": body}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if len(source.Proxies) != 1 {
		t.Fatalf("得到 %d 个节点，期望 1 个", len(source.Proxies))
	}
	proxy := source.Proxies[0]
	if proxy.Name != "HK-AnyTLS" || proxy.Type != "anytls" || proxy.Params["sni"] != "edge.example.com" {
		t.Fatalf("proxy = %+v，期望 AnyTLS 节点，且 QuanX 的 tls-host 映射为 sni", proxy)
	}
}

// T-SRC-003：合并多个订阅
func TestSource_MultiSubscriptionMerge(t *testing.T) {
	body1 := mustReadFile(t, "../../testdata/subscriptions/sample.txt")
	body2 := mustReadFile(t, "../../testdata/subscriptions/sample_sub2.txt")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub1.example.com"},
		{URL: "https://sub2.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub1.example.com": body1,
		"https://sub2.example.com": body2,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	// sample.txt 有 5 个节点，sample_sub2.txt 有 2 个，共 7 个。
	if len(proxies) != 7 {
		t.Fatalf("得到 %d 个节点，期望 7 个", len(proxies))
	}

	// Source 保留声明的名称，并为每个节点分配不同的关系图编号。
	if proxies[0].Name != "HK-01" {
		t.Errorf("第一个 HK-01 的名称 = %q", proxies[0].Name)
	}
	if proxies[5].Name != "HK-01" {
		t.Errorf("重复的 HK-01 在 Source 阶段应保持原名，实际 %q", proxies[5].Name)
	}
	if proxies[6].Name != "KR-01" {
		t.Errorf("KR-01 应保持不变，实际 %q", proxies[6].Name)
	}
	assertSequentialProxyIDs(t, proxies)
}

// T-SRC-ID-001：重复的显示名保持原样，ID 仍保持唯一。
func TestSource_CrossSubscriptionDuplicateNamesGetDistinctIDs(t *testing.T) {
	// 三个订阅都包含 "NODE-A"。
	sub := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@a.example.com:8388#NODE-A")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub1.example.com"},
		{URL: "https://sub2.example.com"},
		{URL: "https://sub3.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub1.example.com": sub,
		"https://sub2.example.com": sub,
		"https://sub3.example.com": sub,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	wantNames := []string{"NODE-A", "NODE-A", "NODE-A"}
	if len(proxies) != len(wantNames) {
		t.Fatalf("得到 %d 个节点，期望 %d 个", len(proxies), len(wantNames))
	}
	for i, want := range wantNames {
		if proxies[i].Name != want {
			t.Errorf("proxy[%d].Name = %q，期望 %q", i, proxies[i].Name, want)
		}
	}
	assertSequentialProxyIDs(t, proxies)
}

func TestSource_CustomProxyConversion(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.CustomProxies = []config.CustomProxy{
		customProxy("HK-ISP", "socks5://user1:pass1@154.197.1.1:45002", nil),
	}

	f := &fakeFetcher{responses: map[string][]byte{}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 1 {
		t.Fatalf("得到 %d 个节点，期望 1 个", len(proxies))
	}

	p := proxies[0]
	if p.Name != "HK-ISP" {
		t.Errorf("Name = %q，期望 %q", p.Name, "HK-ISP")
	}
	if p.Type != "socks5" {
		t.Errorf("Type = %q，期望 %q", p.Type, "socks5")
	}
	if p.Kind != model.KindCustom {
		t.Errorf("Kind = %q，期望 %q", p.Kind, model.KindCustom)
	}
	if p.Params["username"] != "user1" {
		t.Errorf("username = %q，期望 %q", p.Params["username"], "user1")
	}
	if p.Params["password"] != "pass1" {
		t.Errorf("password = %q，期望 %q", p.Params["password"], "pass1")
	}
}

// T-SRC-SS-01：转换 SS 自定义节点时保留 cipher/password/plugin。
func TestSource_CustomProxySSConversion(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.CustomProxies = []config.CustomProxy{
		customProxy("MY-SS", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388", nil),
	}

	f := &fakeFetcher{responses: map[string][]byte{}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 1 {
		t.Fatalf("得到 %d 个节点，期望 1 个", len(proxies))
	}

	p := proxies[0]
	if p.Type != "ss" {
		t.Errorf("Type = %q，期望 ss", p.Type)
	}
	if p.Kind != model.KindCustom {
		t.Errorf("Kind = %q，期望 %q", p.Kind, model.KindCustom)
	}
	if p.Params["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher = %q", p.Params["cipher"])
	}
	if p.Params["password"] != "mypass" {
		t.Errorf("password = %q", p.Params["password"])
	}
	if _, ok := p.Params["username"]; ok {
		t.Error("ss 节点的 Params 中不应有 username")
	}
}

func TestSource_ProxyInvariantValidation(t *testing.T) {
	sources := config.PreparedSources{
		CustomProxies: []config.PreparedCustomProxy{
			{
				Name: "BROKEN-SS",
				Parsed: proxyparse.Result{
					Type:   "ss",
					Server: "1.2.3.4",
					Port:   8388,
					Params: map[string]string{"cipher": "aes-256-gcm"},
				},
			},
		},
	}

	_, err := sourcePrepared(context.Background(), sources, &fakeFetcher{responses: map[string][]byte{}})
	if err == nil {
		t.Fatal("期望不变量错误，实际为 nil")
	}

	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("err 类型 = %T，期望 *errtype.BuildError", err)
	}
	if buildErr.Phase != "source" {
		t.Errorf("Phase = %q，期望 source", buildErr.Phase)
	}
	if !strings.Contains(buildErr.Message, "type=ss 缺少必填参数 password") {
		t.Fatalf("意外的消息：%s", buildErr.Message)
	}
}

// 带 relay_through 的自定义节点是链式模板，不是独立节点。Source 必须在
// convertCustomProxies 中跳过它们，这样 cp.Name 才能用作链式组名，
// 而不会在共享命名空间（代理 / 节点组 / 服务组）中冲突。
func TestSource_CustomProxyWithRelayThroughIsNotConvertedToKindCustom(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.CustomProxies = []config.CustomProxy{
		customProxy("DIRECT-ONLY", "socks5://1.1.1.1:1080", nil),
		customProxy("RELAY-ONLY", "socks5://2.2.2.2:1080", &config.RelayThrough{Type: "all", Strategy: "select"}),
	}

	f := &fakeFetcher{responses: map[string][]byte{}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 1 {
		t.Fatalf("得到 %d 个节点，期望 1 个（必须跳过 RELAY-ONLY）", len(proxies))
	}
	if proxies[0].Name != "DIRECT-ONLY" {
		t.Errorf("保留下来的节点 = %q，期望 DIRECT-ONLY", proxies[0].Name)
	}
	for _, p := range proxies {
		if p.Name == "RELAY-ONLY" {
			t.Errorf("RELAY-ONLY 混入了 Source 输出；作为链式模板的自定义节点不得生成 KindCustom 条目")
		}
	}
}

func TestSource_CustomAndSubscriptionMayShareDisplayName(t *testing.T) {
	sub := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-ISP")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com"},
	}
	cfg.Sources.CustomProxies = []config.CustomProxy{
		customProxy("HK-ISP", "socks5://1.2.3.4:1080", nil),
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": sub,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("相同的显示名应被接受：%v", err)
	}
	if len(source.Proxies) != 2 {
		t.Fatalf("proxies = %+v，期望订阅节点 + 自定义节点", source.Proxies)
	}
	if source.Proxies[0].Name != "HK-ISP" || source.Proxies[1].Name != "HK-ISP" {
		t.Fatalf("节点名称 = %q/%q，期望保持为 HK-ISP", source.Proxies[0].Name, source.Proxies[1].Name)
	}
	assertSequentialProxyIDs(t, source.Proxies)
}

func TestSource_FetchError(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com/api?token=secret"},
	}

	f := &fakeFetcher{err: errors.New("connection refused")}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("期望返回错误")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.FetchError", err)
	}
}

func TestSource_InvalidBase64Response(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": []byte("this is not base64!!!@@@"),
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("无效 base64 应返回错误")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.FetchError", err)
	}
	if fetchErr.Code != errtype.CodeFetchSubscriptionBase64Invalid {
		t.Errorf("Code = %q，期望 %q", fetchErr.Code, errtype.CodeFetchSubscriptionBase64Invalid)
	}
	if !strings.Contains(fetchErr.Message, "Base64") {
		t.Errorf("错误应提到 Base64，实际：%s", fetchErr.Message)
	}
	if !strings.Contains(fetchErr.Message, "SS/AnyTLS 订阅列表") {
		t.Errorf("错误应提到 SS/AnyTLS 订阅列表，实际：%s", fetchErr.Message)
	}
}

// T-SRC-SS-TXT-001：订阅内容可以是明文 SIP002 格式的 ss.txt 列表。
func TestSource_PlainTextSSSubscription(t *testing.T) {
	body := []byte(strings.Join([]string{
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@sg.example.com:8388#SG-01",
	}, "\n"))

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com/ss.txt"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/ss.txt": body,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies
	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}
	wantNames := []string{"HK-01", "SG-01"}
	for i, want := range wantNames {
		if proxies[i].Name != want {
			t.Errorf("proxy[%d].Name = %q，期望 %q", i, proxies[i].Name, want)
		}
		if proxies[i].Kind != model.KindSubscription {
			t.Errorf("proxy[%d].Kind = %q，期望 %q", i, proxies[i].Kind, model.KindSubscription)
		}
	}
}

// T-SRC-SS-TXT-004：订阅内容可以是明文 Quantumult X shadowsocks 列表，
// 节点名写在 tag 中。
func TestSource_PlainTextQuanXSSSubscription(t *testing.T) {
	paramValue := ssLineFixtureValue()
	body := []byte(strings.Join([]string{
		"shadowsocks = demo.com:11127, method=2022-blake3-aes-128-gcm, " + ssLineParam(ssLineParamKey, paramValue) + ", fast-open=false, udp-relay=true, tag=🇭🇰 Hong Kong 01",
		"shadowsocks = demo.com:11558, method=2022-blake3-aes-128-gcm, " + ssLineParam(ssLineParamKey, paramValue) + ", fast-open=false, udp-relay=true, tag=🇭🇰 Hong Kong 02",
	}, "\n"))

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com/quanx-ss.txt"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/quanx-ss.txt": body,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies
	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}
	if proxies[0].Name != "🇭🇰 Hong Kong 01" || proxies[1].Name != "🇭🇰 Hong Kong 02" {
		t.Errorf("节点名称 = [%q, %q]，期望香港节点", proxies[0].Name, proxies[1].Name)
	}
	if proxies[0].Params["cipher"] != "2022-blake3-aes-128-gcm" {
		t.Errorf("cipher = %q", proxies[0].Params["cipher"])
	}
	if proxies[0].Params[ssLineParamKey] != paramValue {
		t.Errorf("认证参数 = %q", proxies[0].Params[ssLineParamKey])
	}
	if proxies[0].Params["udp-relay"] != "true" || proxies[0].Params["tfo"] != "false" {
		t.Errorf("udp-relay/tfo = %q/%q，期望 true/false", proxies[0].Params["udp-relay"], proxies[0].Params["tfo"])
	}
}

// T-SRC-SS-TXT-005：订阅内容可以是明文 Surge SS 节点列表。
// 语法合法的信息条目会被保留；用户如有需要，可以用 filters.exclude 过滤掉。
func TestSource_PlainTextSurgeSSSubscription(t *testing.T) {
	paramValue := ssLineFixtureValue()
	body := []byte(strings.Join([]string{
		"Traffic Reset：29 Days Left= ss, demo.com, 11127, encrypt-method=2022-blake3-aes-128-gcm, " + quotedSSLineParam(ssLineParamKey, paramValue) + ", udp-relay=true, tfo=false",
		"Expire Date：2026/06/05= ss, demo.com, 11127, encrypt-method=2022-blake3-aes-128-gcm, " + quotedSSLineParam(ssLineParamKey, paramValue) + ", udp-relay=true, tfo=false",
		"🇭🇰 Hong Kong 01= ss, demo.com, 11127, encrypt-method=2022-blake3-aes-128-gcm, " + quotedSSLineParam(ssLineParamKey, paramValue) + ", udp-relay=true, tfo=false",
	}, "\n"))

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com/surge-ss.txt"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/surge-ss.txt": body,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies
	if len(proxies) != 3 {
		t.Fatalf("得到 %d 个节点，期望 3 个", len(proxies))
	}
	if proxies[0].Name != "Traffic Reset：29 Days Left" {
		t.Errorf("proxy[0].Name = %q，期望保留信息条目", proxies[0].Name)
	}
	if proxies[2].Name != "🇭🇰 Hong Kong 01" {
		t.Errorf("proxy[2].Name = %q，期望香港节点", proxies[2].Name)
	}
	if proxies[2].Params["udp-relay"] != "true" || proxies[2].Params["tfo"] != "false" {
		t.Errorf("udp-relay/tfo = %q/%q，期望 true/false", proxies[2].Params["udp-relay"], proxies[2].Params["tfo"])
	}
}

// T-SRC-SS-TXT-002：明文订阅跳过注释、空行和格式错误的条目，
// 保留有效的 SS 节点。
func TestSource_PlainTextSSSubscriptionSkipsInvalidLines(t *testing.T) {
	body := []byte(strings.Join([]string{
		"# comment",
		"",
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01",
		"not-a-valid-ss-uri",
		"// another comment",
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@sg.example.com:8388#SG-01",
	}, "\n"))

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com/ss.txt"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/ss.txt": body,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies
	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}
	if proxies[0].Name != "HK-01" || proxies[1].Name != "SG-01" {
		t.Errorf("节点名称 = [%q, %q]，期望 [HK-01, SG-01]", proxies[0].Name, proxies[1].Name)
	}
}

// T-SRC-SS-TXT-003：明文订阅中没有有效 SS 节点时，
// 报告与 base64 订阅相同的空订阅错误。
func TestSource_PlainTextSSSubscriptionOnlyInvalidLinesIsEmpty(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com/ss.txt"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/ss.txt": []byte("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:0#BAD"),
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("空的明文订阅应返回错误")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.FetchError", err)
	}
	if fetchErr.Code != errtype.CodeFetchSubscriptionEmpty {
		t.Errorf("Code = %q，期望 %q", fetchErr.Code, errtype.CodeFetchSubscriptionEmpty)
	}
}

func TestSource_EmptySubscription(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": []byte(""),
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("空订阅应返回错误")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.FetchError", err)
	}
	if fetchErr.URL != "https://sub.example.com" {
		t.Errorf("URL = %q，期望 %q", fetchErr.URL, "https://sub.example.com")
	}
	if !strings.Contains(fetchErr.Message, "任何有效节点") {
		t.Errorf("错误应提到订阅为空，实际：%s", fetchErr.Message)
	}
}

func TestSource_MixedValidInvalidURIs(t *testing.T) {
	sub := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01",
		"not-a-valid-ss-uri",
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@sg.example.com:8388#SG-01",
	)

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": sub,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	// 无效行应被跳过，剩下 2 个有效节点。
	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}
	if proxies[0].Name != "HK-01" {
		t.Errorf("proxy[0].Name = %q，期望 %q", proxies[0].Name, "HK-01")
	}
	if proxies[1].Name != "SG-01" {
		t.Errorf("proxy[1].Name = %q，期望 %q", proxies[1].Name, "SG-01")
	}
}

func TestSource_SubscriptionAndCustomCombined(t *testing.T) {
	sub := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com"},
	}
	cfg.Sources.CustomProxies = []config.CustomProxy{
		customProxy("MY-PROXY", "http://10.0.0.1:8080", nil),
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": sub,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	// 1 个订阅节点 + 1 个自定义节点，共 2 个。
	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}

	// 订阅节点在前，自定义节点在后。
	if proxies[0].Kind != model.KindSubscription {
		t.Errorf("proxy[0].Kind = %q，期望 subscription", proxies[0].Kind)
	}
	if proxies[1].Kind != model.KindCustom {
		t.Errorf("proxy[1].Kind = %q，期望 custom", proxies[1].Kind)
	}
}

func TestSource_NaturalSuffixDoesNotTriggerSourceRenaming(t *testing.T) {
	// 后缀由各目标格式内的命名阶段分配；Source 保留全部三个声明的名称，
	// 只分配稳定的关系图编号。
	sub := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@a.example.com:8388#NODE-A",
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@b.example.com:8388#NODE-A%E2%91%A1", // NODE-A②
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@c.example.com:8388#NODE-A",
	)

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{
		{URL: "https://sub.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": sub,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 3 {
		t.Fatalf("得到 %d 个节点，期望 3 个", len(proxies))
	}

	wantNames := []string{"NODE-A", "NODE-A②", "NODE-A"}
	for index, want := range wantNames {
		if proxies[index].Name != want {
			t.Errorf("proxy[%d].Name = %q，期望 %q", index, proxies[index].Name, want)
		}
	}
	assertSequentialProxyIDs(t, proxies)
}

func TestSource_NoSubscriptionsOnlyCustom(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.CustomProxies = []config.CustomProxy{
		customProxy("PROXY-1", "socks5://1.1.1.1:1080", nil),
	}

	f := &fakeFetcher{responses: map[string][]byte{}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 1 {
		t.Fatalf("得到 %d 个节点，期望 1 个", len(proxies))
	}
	if proxies[0].Name != "PROXY-1" {
		t.Errorf("Name = %q，期望 PROXY-1", proxies[0].Name)
	}
}

// T-SRC-SNELL-001：Snell 来源产出 Kind=KindSnell 的节点。
func TestSource_SnellSource(t *testing.T) {
	body := []byte("HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4, reuse=true\n" +
		"# comment line\n" +
		"\n" +
		"SG-Snell = snell, 5.6.7.8, 8989, psk=yyy, version=4\n")
	cfg := baseCfg()
	cfg.Sources.Snell = []config.Source{
		{URL: "https://example.com/snell-nodes.txt"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/snell-nodes.txt": body,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}
	if proxies[0].Name != "HK-Snell" || proxies[0].Kind != model.KindSnell {
		t.Errorf("proxies[0] = {Name: %q, Kind: %q}，期望 {HK-Snell, snell}", proxies[0].Name, proxies[0].Kind)
	}
	if proxies[1].Name != "SG-Snell" || proxies[1].Type != "snell" {
		t.Errorf("proxies[1] = {Name: %q, Type: %q}，期望 {SG-Snell, snell}", proxies[1].Name, proxies[1].Type)
	}
	if proxies[0].Params["psk"] != "xxx" {
		t.Errorf("proxies[0].Params[psk] = %q，期望 xxx", proxies[0].Params["psk"])
	}
}

// T-SRC-SNELL-002：订阅节点与 Snell 节点的名称可以重复，ID 不会重复。
func TestSource_SnellAndSubscriptionDuplicateNamesGetDistinctIDs(t *testing.T) {
	// 名为 "HK-01" 的 SS 订阅节点。
	subBody := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01")
	// 同名的 Snell 来源节点。
	snellBody := []byte("HK-01 = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{{URL: "https://sub.example.com/api"}}
	cfg.Sources.Snell = []config.Source{{URL: "https://example.com/snell.txt"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/api":   subBody,
		"https://example.com/snell.txt": snellBody,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}
	if proxies[0].Name != "HK-01" {
		t.Errorf("proxies[0].Name = %q，期望 HK-01", proxies[0].Name)
	}
	if proxies[1].Name != "HK-01" {
		t.Errorf("proxies[1].Name = %q，期望保持原名 HK-01", proxies[1].Name)
	}
	assertSequentialProxyIDs(t, proxies)
}

// T-SRC-SNELL-MULTI：多个 Snell 来源保留节点名称，并按声明顺序分配 ID。
func TestSource_MultiSnellDuplicateNamesGetDistinctIDs(t *testing.T) {
	url1 := "https://snell.example.com/a.txt"
	url2 := "https://snell.example.com/b.txt"
	body1 := []byte("SG-01 = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")
	body2 := []byte("SG-01 = snell, 5.6.7.8, 8989, psk=yyy, version=4\n")

	cfg := baseCfg()
	cfg.Sources.Snell = []config.Source{{URL: url1}, {URL: url2}}

	f := &fakeFetcher{responses: map[string][]byte{
		url1: body1,
		url2: body2,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}
	if proxies[0].Name != "SG-01" {
		t.Errorf("proxies[0].Name = %q，期望 SG-01", proxies[0].Name)
	}
	if proxies[1].Name != "SG-01" {
		t.Errorf("proxies[1].Name = %q，期望保持原名 SG-01", proxies[1].Name)
	}
	assertSequentialProxyIDs(t, proxies)
	// 两者都是 KindSnell。
	for i, px := range proxies {
		if px.Kind != model.KindSnell {
			t.Errorf("proxies[%d].Kind = %q，期望 snell", i, px.Kind)
		}
	}
}

// T-SRC-SNELL-003：Snell 行格式错误时立即失败（不静默跳过）。
func TestSource_SnellSource_MalformedLineFailsFast(t *testing.T) {
	body := []byte("# comment\n" +
		"\n" +
		"HK = snell, 1.2.3.4, 57891, psk=xxx, version=4\n" +
		"INVALID = snell, 1.2.3.4, no-port-here\n") // 格式错误

	cfg := baseCfg()
	cfg.Sources.Snell = []config.Source{{URL: "https://example.com/snell.txt?token=secret"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/snell.txt?token=secret": body,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("格式错误的 Snell 行应返回错误")
	}
	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("err 类型 = %T，期望 *errtype.BuildError", err)
	}
	if buildErr.Code != errtype.CodeBuildSnellLineInvalid {
		t.Errorf("Code = %q，期望 %q", buildErr.Code, errtype.CodeBuildSnellLineInvalid)
	}
	if !strings.Contains(buildErr.Message, `Snell 来源 "https://example.com/snell.txt" 第 4 行解析失败`) {
		t.Errorf("消息应包含脱敏后的 URL 和物理行号，实际：%s", buildErr.Message)
	}
	if strings.Contains(buildErr.Message, "secret") {
		t.Errorf("消息泄露了秘密令牌：%s", buildErr.Message)
	}
	if strings.Contains(buildErr.Message, "构建错误 [source]") {
		t.Errorf("消息应内嵌内层错误详情，且不带构建错误前缀：%s", buildErr.Message)
	}

	inner := errors.Unwrap(buildErr)
	var innerBuildErr *errtype.BuildError
	if !errors.As(inner, &innerBuildErr) {
		t.Fatalf("内层 err 类型 = %T，期望 *errtype.BuildError", inner)
	}
	if innerBuildErr == buildErr {
		t.Fatal("外层 BuildError 应包装内层解析错误，而不是包装自身")
	}
	if innerBuildErr.Code != errtype.CodeBuildSnellLineInvalid {
		t.Errorf("内层 Code = %q，期望 %q", innerBuildErr.Code, errtype.CodeBuildSnellLineInvalid)
	}
	if !strings.Contains(innerBuildErr.Message, `port "no-port-here" 不是整数`) {
		t.Errorf("内层消息应保留解析器给出的详情，实际：%s", innerBuildErr.Message)
	}
}

// T-SRC-SNELL-004：Snell 来源为空（所有行都被注释掉）时返回 FetchError。
func TestSource_SnellSource_EmptyReported(t *testing.T) {
	body := []byte("# only comments here\n\n// nothing else\n")

	cfg := baseCfg()
	cfg.Sources.Snell = []config.Source{{URL: "https://example.com/snell.txt?token=secret"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/snell.txt?token=secret": body,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("空的 Snell 来源应返回错误")
	}
	var fe *errtype.FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("err 类型 = %T，期望 *errtype.FetchError", err)
	}
	if fe.Code != errtype.CodeFetchSubscriptionEmpty {
		t.Errorf("Code = %q，期望 %q", fe.Code, errtype.CodeFetchSubscriptionEmpty)
	}
	// 错误信息中的 URL 必须经过脱敏（不含原始令牌）。
	if strings.Contains(fe.URL, "secret") {
		t.Errorf("FetchError.URL 泄露了秘密信息：%q", fe.URL)
	}
}

// --- VLESS 来源测试 ---

// vlessURIFor 为测试 fixture 构造一个最小的合法 vless:// URI。
func vlessURIFor(name, server string, port int) string {
	return fmt.Sprintf("vless://11111111-2222-3333-4444-555555555555@%s:%d?security=tls&sni=%s&type=tcp#%s",
		server, port, server, name)
}

// T-SRC-VLESS-SRC-001：基本的 VLESS 来源产出正确的节点，并跳过其中的注释和空行。
func TestSource_VLessSourceBasic(t *testing.T) {
	body := []byte("# vless nodes\n" +
		"\n" +
		vlessURIFor("HK-VL", "hk.example.com", 443) + "\n" +
		"// second block\n" +
		vlessURIFor("SG-VL", "sg.example.com", 443) + "\n")

	cfg := baseCfg()
	cfg.Sources.VLess = []config.Source{{URL: "https://example.com/vless.txt"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/vless.txt": body,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(proxies))
	}
	if proxies[0].Name != "HK-VL" || proxies[0].Kind != model.KindVLess {
		t.Errorf("proxies[0] = {%q, %q}，期望 {HK-VL, vless}", proxies[0].Name, proxies[0].Kind)
	}
	if proxies[1].Name != "SG-VL" || proxies[1].Type != "vless" {
		t.Errorf("proxies[1] = {%q, %q}，期望 {SG-VL, vless}", proxies[1].Name, proxies[1].Type)
	}
}

// T-SRC-VLESS-SRC-002：未知传输方式回退为 tcp，非 none 的 encryption
// 在来源解析后得以保留。
func TestSource_VLessUnknownTypeFallsBackAndEncryptionPassesThrough(t *testing.T) {
	body := []byte(
		"vless://11111111-2222-3333-4444-555555555555@hk.example.com:443?security=tls&sni=hk.example.com&type=quic&encryption=mlkem768x25519plus.native#HK-VL\n",
	)

	cfg := baseCfg()
	cfg.Sources.VLess = []config.Source{{URL: "https://example.com/vless.txt"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/vless.txt": body,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies
	if len(proxies) != 1 {
		t.Fatalf("得到 %d 个节点，期望 1 个", len(proxies))
	}
	if proxies[0].Params["network"] != "tcp" {
		t.Errorf("Params[network] = %q，期望回退为 tcp", proxies[0].Params["network"])
	}
	if proxies[0].Params["encryption"] != "mlkem768x25519plus.native" {
		t.Errorf("Params[encryption] = %q，期望原样透传", proxies[0].Params["encryption"])
	}
}

// T-SRC-VLESS-SRC-003：VLESS 行格式错误时立即失败（与 Snell 一样采用严格模式）。
func TestSource_VLessBadLineAbortsSource(t *testing.T) {
	body := []byte("# header\n" +
		vlessURIFor("HK-VL", "hk.example.com", 443) + "\n" +
		"vless://not-a-uuid@example.com:443?type=tcp#bad\n") // uuid 无效

	cfg := baseCfg()
	cfg.Sources.VLess = []config.Source{{URL: "https://example.com/vless.txt?token=secret"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/vless.txt?token=secret": body,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("格式错误的 VLESS 行应返回错误")
	}
	var be *errtype.BuildError
	if !errors.As(err, &be) {
		t.Fatalf("err 类型 = %T，期望 *errtype.BuildError", err)
	}
	if be.Code != errtype.CodeBuildVLessSourceLineInvalid {
		t.Errorf("Code = %q，期望 %q", be.Code, errtype.CodeBuildVLessSourceLineInvalid)
	}
	// Unwrap：内层 err 是单条 URI 的解析错误，错误码为 CodeBuildVLessURIInvalid。
	var inner *errtype.BuildError
	if !errors.As(errors.Unwrap(be), &inner) {
		t.Fatalf("内层 err 不是 BuildError")
	}
	if inner.Code != errtype.CodeBuildVLessURIInvalid {
		t.Errorf("内层 Code = %q，期望 %q", inner.Code, errtype.CodeBuildVLessURIInvalid)
	}
	if !strings.Contains(be.Message, `VLESS 来源 "https://example.com/vless.txt" 第 3 行解析失败`) {
		t.Errorf("消息应包含脱敏后的 URL 和行号，实际：%s", be.Message)
	}
	if strings.Contains(be.Message, "secret") {
		t.Errorf("消息泄露了秘密令牌：%s", be.Message)
	}
}

// T-SRC-VLESS-SRC-004：VLESS 来源全是注释时报告为空。
func TestSource_VLessSource_EmptyReported(t *testing.T) {
	body := []byte("# only comments\n\n// nothing else\n")

	cfg := baseCfg()
	cfg.Sources.VLess = []config.Source{{URL: "https://example.com/vless.txt?token=secret"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/vless.txt?token=secret": body,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("空的 VLESS 来源应返回错误")
	}
	var fe *errtype.FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("err 类型 = %T，期望 *errtype.FetchError", err)
	}
	if fe.Code != errtype.CodeFetchSubscriptionEmpty {
		t.Errorf("Code = %q，期望 %q", fe.Code, errtype.CodeFetchSubscriptionEmpty)
	}
	if !strings.Contains(fe.Message, "VLESS") {
		t.Errorf("消息应提到 VLESS，实际：%s", fe.Message)
	}
	if strings.Contains(fe.URL, "secret") {
		t.Errorf("FetchError.URL 泄露了秘密信息：%q", fe.URL)
	}
}

// T-SRC-VLESS-SRC-005：订阅、Snell 和 VLESS 节点可以使用相同的显示名，
// 但按 ID 仍是不同的节点。
func TestSource_VLessSharesDisplayNameWithSSAndSnell(t *testing.T) {
	subBody := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01")
	snellBody := []byte("HK-01 = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")
	vlessBody := []byte(vlessURIFor("HK-01", "hk.example.com", 443) + "\n")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Source{{URL: "https://sub.example.com/api"}}
	cfg.Sources.Snell = []config.Source{{URL: "https://snell.example.com/s.txt"}}
	cfg.Sources.VLess = []config.Source{{URL: "https://vless.example.com/v.txt"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/api":     subBody,
		"https://snell.example.com/s.txt": snellBody,
		"https://vless.example.com/v.txt": vlessBody,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	proxies := source.Proxies

	if len(proxies) != 3 {
		t.Fatalf("得到 %d 个节点，期望 3 个", len(proxies))
	}
	wantNames := []string{"HK-01", "HK-01", "HK-01"}
	for i, want := range wantNames {
		if proxies[i].Name != want {
			t.Errorf("proxies[%d].Name = %q，期望 %q", i, proxies[i].Name, want)
		}
	}
	assertSequentialProxyIDs(t, proxies)
}

// T-SRC-VLESS-SRC-006：VLESS 节点与自定义节点可以使用相同的显示名。
func TestSource_VLessAndCustomDuplicateNamesGetDistinctIDs(t *testing.T) {
	vlessBody := []byte(vlessURIFor("HK-01", "hk.example.com", 443) + "\n")

	cfg := baseCfg()
	cfg.Sources.VLess = []config.Source{{URL: "https://vless.example.com/v.txt"}}
	cfg.Sources.CustomProxies = []config.CustomProxy{{
		Name: "HK-01",
		URL:  "socks5://1.2.3.4:1080",
	}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://vless.example.com/v.txt": vlessBody,
	}}

	source, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("相同的显示名应被接受：%v", err)
	}
	if len(source.Proxies) != 2 || source.Proxies[0].Name != "HK-01" || source.Proxies[1].Name != "HK-01" {
		t.Fatalf("proxies = %+v，期望两个 HK-01 节点", source.Proxies)
	}
	assertSequentialProxyIDs(t, source.Proxies)
}

// TestSource_UnknownFetchKindErrors 守护 Source() 中 switch 的 default 分支：
// 如果 Sources.FetchOrder 含有拼错或不支持的类型，Source() 返回 BuildError，
// 而不是静默跳过。经由 YAML 的正式路径走不到这里（UnmarshalYAML 会提前拒绝
// 未知键），但测试或今后的扩展在内存中直接构造 Config 时可能走到。
func TestSource_UnknownFetchKindErrors(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.FetchOrder = []string{"hysteria"} // 拼写错误 / 未来的类型

	f := &fakeFetcher{}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("未知的拉取类型应返回错误，实际为 nil")
	}
	var be *errtype.BuildError
	if !errors.As(err, &be) {
		t.Fatalf("err 类型 = %T，期望 *errtype.BuildError", err)
	}
	if be.Code != errtype.CodeBuildValidationFailed {
		t.Errorf("Code = %q，期望 %q", be.Code, errtype.CodeBuildValidationFailed)
	}
	if !strings.Contains(be.Message, `"hysteria"`) {
		t.Errorf("消息应引用出错的类型，实际：%s", be.Message)
	}
}

// T-SRC-VLESS-SRC-007：Source 按 Sources.FetchOrder 的顺序遍历来源。
//
// 按 snell→vless→subscriptions 的顺序声明，得到的节点列表也应按同样的类别
// 顺序排列。测试在不同的 FetchOrder 排列下检查实际顺序，证明 switch 分派
// 遵循用户 YAML 中的书写顺序。
func TestSource_RespectsFetchOrder(t *testing.T) {
	subBody := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@sub.example.com:8388#SS-01")
	snellBody := []byte("SNELL-01 = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")
	vlessBody := []byte(vlessURIFor("VL-01", "v.example.com", 443) + "\n")

	cases := []struct {
		name  string
		order []string
		want  []string
	}{
		{"snell first", []string{"snell", "vless", "subscriptions"}, []string{"SNELL-01", "VL-01", "SS-01"}},
		{"vless first", []string{"vless", "subscriptions", "snell"}, []string{"VL-01", "SS-01", "SNELL-01"}},
		{"subs first", []string{"subscriptions", "snell", "vless"}, []string{"SS-01", "SNELL-01", "VL-01"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseCfg()
			cfg.Sources.Subscriptions = []config.Source{{URL: "https://sub.example.com/api"}}
			cfg.Sources.Snell = []config.Source{{URL: "https://snell.example.com/s.txt"}}
			cfg.Sources.VLess = []config.Source{{URL: "https://vless.example.com/v.txt"}}
			cfg.Sources.FetchOrder = tc.order

			f := &fakeFetcher{responses: map[string][]byte{
				"https://sub.example.com/api":     subBody,
				"https://snell.example.com/s.txt": snellBody,
				"https://vless.example.com/v.txt": vlessBody,
			}}

			source, err := Source(context.Background(), cfg, f)
			if err != nil {
				t.Fatalf("意外错误：%v", err)
			}
			proxies := source.Proxies
			if len(proxies) != 3 {
				t.Fatalf("得到 %d 个节点，期望 3 个", len(proxies))
			}
			for i, want := range tc.want {
				if proxies[i].Name != want {
					t.Errorf("proxies[%d].Name = %q，期望 %q", i, proxies[i].Name, want)
				}
			}
		})
	}
}

// relay_through 的名称会成为一个分组，而拉取类节点在内部用 ID 标识；
// 两者最终在输出中的名称冲突由目标格式投影阶段解决。
func TestSource_ChainGroupAndFetchedNodeMayShareDisplayName(t *testing.T) {
	const sharedName = "HK-EDGE"

	subBody := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#" + sharedName)
	snellBody := []byte(sharedName + " = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")
	vlessBody := []byte(vlessURIFor(sharedName, "hk.example.com", 443) + "\n")

	cases := []struct {
		name      string
		setupCfg  func(*config.Config)
		responses map[string][]byte
	}{
		{
			name: "subscription",
			setupCfg: func(cfg *config.Config) {
				cfg.Sources.Subscriptions = []config.Source{{URL: "https://sub.example.com/api"}}
			},
			responses: map[string][]byte{"https://sub.example.com/api": subBody},
		},
		{
			name: "snell",
			setupCfg: func(cfg *config.Config) {
				cfg.Sources.Snell = []config.Source{{URL: "https://snell.example.com/s.txt"}}
			},
			responses: map[string][]byte{"https://snell.example.com/s.txt": snellBody},
		},
		{
			name: "vless",
			setupCfg: func(cfg *config.Config) {
				cfg.Sources.VLess = []config.Source{{URL: "https://vless.example.com/v.txt"}}
			},
			responses: map[string][]byte{"https://vless.example.com/v.txt": vlessBody},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseCfg()
			tc.setupCfg(cfg)
			cfg.Sources.CustomProxies = []config.CustomProxy{{
				Name:         sharedName,
				URL:          "socks5://1.1.1.1:1080",
				RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
			}}

			f := &fakeFetcher{responses: tc.responses}

			source, err := Source(context.Background(), cfg, f)
			if err != nil {
				t.Fatalf("相同的显示名应被接受：%v", err)
			}
			if len(source.Proxies) != 1 || source.Proxies[0].Name != sharedName {
				t.Fatalf("proxies = %+v，期望拉取到的 %s", source.Proxies, sharedName)
			}
			if len(source.ChainTemplates) != 1 || source.ChainTemplates[0].Name != sharedName {
				t.Fatalf("链式模板 = %+v，期望 %s", source.ChainTemplates, sharedName)
			}
		})
	}
}
