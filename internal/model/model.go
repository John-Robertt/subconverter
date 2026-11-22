package model

// ProxyID 是代理在单次流水线构建中的身份标识。它在代理创建时分配一次
// （原始节点由 Source 分配，链式节点由 Group 分配），在关系图被过滤和
// 投影的过程中保持不变。它不会跨配置版本保留。
type ProxyID uint64

// ProxyKind 标记代理节点的来源。
type ProxyKind string

const (
	KindSubscription ProxyKind = "subscription"
	KindSnell        ProxyKind = "snell"
	KindVLess        ProxyKind = "vless"
	KindCustom       ProxyKind = "custom"
	KindChained      ProxyKind = "chained"
)

// NativeProxyFormat 标识从结构化订阅文档原样复制过来的目标原生代理声明
// 属于哪种格式。
type NativeProxyFormat string

const (
	NativeProxyFormatClash NativeProxyFormat = "clash"
	NativeProxyFormatSurge NativeProxyFormat = "surge"
	NativeProxyFormatLoon  NativeProxyFormat = "loon"
)

// NativeProxy 保存一条已经用某个目标客户端原生语法写好的代理声明。
// Payload 是一个 Clash YAML 代理映射，或者一条 Surge/Loon [Proxy]
// 声明中等号右侧的内容。
type NativeProxy struct {
	Format  NativeProxyFormat
	Payload string
}

// Proxy 表示中间表示中的单个代理节点。
type Proxy struct {
	ID              ProxyID
	Name            string // 声明的展示名候选；从不用作身份标识
	Type            string // 例如 "ss"、"anytls"、"socks5"、"http"、"snell"、"vless"
	Server          string
	Port            int
	Params          map[string]string // 协议相关参数（cipher、password 等）
	Plugin          *Plugin
	Kind            ProxyKind
	DialerID        ProxyID // 上游代理的身份标识；只有链式节点会设置
	ChainNameSuffix string  // 链式模板名，拼接在目标客户端内的上游节点名之后
	Native          *NativeProxy
}

// Plugin 表示挂在代理上的出站插件。
type Plugin struct {
	Name string
	Opts map[string]string
}

// NodeGroup 只包含代理身份标识。把它和 RouteGroup 分开，
// 可以防止代理名字重新变成关系图里的引用。
type NodeGroup struct {
	Name     string
	Strategy string    // "select" 或 "url-test"
	Members  []ProxyID // 有序的代理身份标识
}

// PolicyRefKind 标识路由组中某个成员指向的目标类型。
type PolicyRefKind string

const (
	PolicyRefProxy      PolicyRefKind = "proxy"
	PolicyRefNodeGroup  PolicyRefKind = "node_group"
	PolicyRefRouteGroup PolicyRefKind = "route_group"
	PolicyRefReserved   PolicyRefKind = "reserved"
)

// PolicyRef 是带类型的路由组成员引用。代理引用使用 ProxyID；
// 分组引用和保留策略引用使用 Name。
type PolicyRef struct {
	Kind    PolicyRefKind
	ProxyID ProxyID
	Name    string
}

// RouteGroup 表示服务层级的分组，成员可以引用代理、节点组、
// 路由组或保留策略。
type RouteGroup struct {
	Name     string
	Strategy string
	Members  []PolicyRef
}

// Ruleset 把一组远程规则 URL 绑定到一个路由策略。
type Ruleset struct {
	Policy string
	URLs   []string
}

// Rule 表示单条内联规则。
type Rule struct {
	Raw    string // 原始字符串，例如 "GEOIP,CN,🎯 China"
	Policy string // 取自最后一个逗号分隔段
}

// Pipeline 是 Build 生成的、与输出格式无关的 ID 关系图。
type Pipeline struct {
	Proxies     []Proxy
	NodeGroups  []NodeGroup  // 区域组和链式组，有序
	RouteGroups []RouteGroup // 服务组，有序
	Rulesets    []Ruleset    // 有序
	Rules       []Rule       // 有序
	Fallback    string       // fallback 路由组的名字
	AllProxies  []ProxyID    // @all 的展开结果：只包含原始节点的身份标识
}

// TargetProxy 是经过目标过滤、并在目标客户端内分配好名字的代理。
// 渲染器使用这个类型，不会接触内部编号。
type TargetProxy struct {
	Name   string
	Type   string
	Server string
	Port   int
	Params map[string]string
	Plugin *Plugin
	Kind   ProxyKind
	Dialer string
	Native *NativeProxy
}

// TargetGroup 是交给渲染器的分组，成员已是最终名字。
type TargetGroup struct {
	Name     string
	Strategy string
	Members  []string
}

// TargetChain 是目标客户端原生的代理链，成员按跳转顺序列出最终策略名。
type TargetChain struct {
	Name    string
	Members []string
	UDP     bool
}

// TargetPipeline 是 Pipeline 投影到目标客户端后、交给渲染器的结果。
type TargetPipeline struct {
	ProxyChains []TargetChain
	Proxies     []TargetProxy
	NodeGroups  []TargetGroup
	RouteGroups []TargetGroup
	Rulesets    []Ruleset
	Rules       []Rule
	Fallback    string
}
