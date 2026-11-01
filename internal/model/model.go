package model

// ProxyID is the identity of a proxy inside one pipeline build. It is assigned
// once when the proxy is created (Source for original proxies, Group for
// chained proxies) and remains stable while the graph is filtered and
// projected. It is not persistent across configuration revisions.
type ProxyID uint64

// ProxyKind indicates the origin of a proxy node.
type ProxyKind string

const (
	KindSubscription ProxyKind = "subscription"
	KindSnell        ProxyKind = "snell"
	KindVLess        ProxyKind = "vless"
	KindCustom       ProxyKind = "custom"
	KindChained      ProxyKind = "chained"
)

// NativeProxyFormat identifies a target-native proxy declaration copied from
// a structured subscription document.
type NativeProxyFormat string

const (
	NativeProxyFormatClash NativeProxyFormat = "clash"
	NativeProxyFormatSurge NativeProxyFormat = "surge"
	NativeProxyFormatLoon  NativeProxyFormat = "loon"
)

// NativeProxy preserves a proxy declaration that is already expressed in one
// target's native syntax. Payload contains one Clash YAML proxy mapping or the
// right-hand side of one Surge or Loon [Proxy] declaration.
type NativeProxy struct {
	Format  NativeProxyFormat
	Payload string
}

// Proxy represents a single proxy node in the intermediate representation.
type Proxy struct {
	ID              ProxyID
	Name            string // declared display-name candidate; never used as identity
	Type            string // e.g. "ss", "anytls", "socks5", "http", "snell", "vless"
	Server          string
	Port            int
	Params          map[string]string // type-specific parameters (cipher, password, etc.)
	Plugin          *Plugin
	Kind            ProxyKind
	DialerID        ProxyID // upstream proxy identity; only set for chained proxies
	ChainNameSuffix string  // chain template name appended after the target-local dialer name
	Native          *NativeProxy
}

// Plugin represents an outbound plugin attached to a proxy.
type Plugin struct {
	Name string
	Opts map[string]string
}

// NodeGroup contains only proxy identities. Keeping it separate from
// RouteGroup prevents proxy names from becoming graph references again.
type NodeGroup struct {
	Name     string
	Strategy string    // "select" or "url-test"
	Members  []ProxyID // ordered proxy identities
}

// PolicyRefKind identifies the target of one route-group member.
type PolicyRefKind string

const (
	PolicyRefProxy      PolicyRefKind = "proxy"
	PolicyRefNodeGroup  PolicyRefKind = "node_group"
	PolicyRefRouteGroup PolicyRefKind = "route_group"
	PolicyRefReserved   PolicyRefKind = "reserved"
)

// PolicyRef is a typed route-group reference. Proxy references use ProxyID;
// group and reserved-policy references use Name.
type PolicyRef struct {
	Kind    PolicyRefKind
	ProxyID ProxyID
	Name    string
}

// RouteGroup represents a service-level group whose members may refer to
// proxies, node groups, route groups, or reserved policies.
type RouteGroup struct {
	Name     string
	Strategy string
	Members  []PolicyRef
}

// Ruleset binds a set of remote rule URLs to a routing policy.
type Ruleset struct {
	Policy string
	URLs   []string
}

// Rule represents a single inline rule entry.
type Rule struct {
	Raw    string // original string, e.g. "GEOIP,CN,🎯 China"
	Policy string // extracted from last comma-delimited segment
}

// Pipeline is the format-agnostic ID graph produced by Build.
type Pipeline struct {
	Proxies     []Proxy
	NodeGroups  []NodeGroup  // region groups + chain groups, ordered
	RouteGroups []RouteGroup // service groups, ordered
	Rulesets    []Ruleset    // ordered
	Rules       []Rule       // ordered
	Fallback    string       // name of the fallback route group
	AllProxies  []ProxyID    // @all expansion: original proxy identities only
}

// TargetProxy is a proxy after target filtering and target-local name
// allocation. Renderers consume this type and never observe internal IDs.
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

// TargetGroup is a renderer-facing group with final member names.
type TargetGroup struct {
	Name     string
	Strategy string
	Members  []string
}

// TargetChain is a target-native chain with final policy names in hop order.
type TargetChain struct {
	Name    string
	Members []string
	UDP     bool
}

// TargetPipeline is the renderer-facing projection of Pipeline.
type TargetPipeline struct {
	ProxyChains []TargetChain
	Proxies     []TargetProxy
	NodeGroups  []TargetGroup
	RouteGroups []TargetGroup
	Rulesets    []Ruleset
	Rules       []Rule
	Fallback    string
}
