package model

// ProxyKind indicates the origin of a proxy node.
type ProxyKind string

const (
	KindSubscription ProxyKind = "subscription"
	KindCustom       ProxyKind = "custom"
	KindChained      ProxyKind = "chained"
)

// GroupScope indicates whether a group is node-level or route-level.
type GroupScope string

const (
	ScopeNode  GroupScope = "node"
	ScopeRoute GroupScope = "route"
)

// Proxy represents a single proxy node in the intermediate representation.
type Proxy struct {
	Name   string
	Type   string // e.g. "ss", "socks5", "http"
	Server string
	Port   int
	Params map[string]string // type-specific parameters (cipher, password, etc.)
	Plugin *Plugin
	Kind   ProxyKind
	Dialer string // upstream proxy name; only set for chained proxies
}

// Plugin represents an outbound plugin attached to a proxy.
type Plugin struct {
	Name string
	Opts map[string]string
}

// ProxyGroup represents a named group of proxies on the client panel.
type ProxyGroup struct {
	Name     string
	Scope    GroupScope
	Strategy string   // "select" or "url-test"
	Members  []string // ordered list of member names
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

// Pipeline is the complete intermediate result of one generation pass.
// It aggregates all entities needed by renderers.
type Pipeline struct {
	Proxies     []Proxy
	NodeGroups  []ProxyGroup // region groups + chain groups, ordered
	RouteGroups []ProxyGroup // service groups, ordered
	Rulesets    []Ruleset    // ordered
	Rules       []Rule       // ordered
	Fallback    string       // name of the fallback route group
	AllProxies  []string     // @all expansion: original proxy names only
}
