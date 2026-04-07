package config

// Config is the top-level user configuration.
type Config struct {
	Sources  Sources              `yaml:"sources"`
	Filters  Filters              `yaml:"filters"`
	Groups   OrderedMap[Group]    `yaml:"groups"`
	Routing  OrderedMap[[]string] `yaml:"routing"`
	Rulesets OrderedMap[[]string] `yaml:"rulesets"`
	Rules    []string             `yaml:"rules"`
	Fallback string               `yaml:"fallback"`
	BaseURL  string               `yaml:"base_url,omitempty"`
}

// Sources declares subscription and custom proxy inputs.
type Sources struct {
	Subscriptions []Subscription `yaml:"subscriptions"`
	CustomProxies []CustomProxy  `yaml:"custom_proxies"`
}

// Subscription is a single subscription source.
type Subscription struct {
	URL string `yaml:"url"`
}

// CustomProxy is a user-defined proxy node.
type CustomProxy struct {
	Name         string        `yaml:"name"`
	Type         string        `yaml:"type"`                    // "socks5" | "http"
	Server       string        `yaml:"server"`
	Port         int           `yaml:"port"`
	Username     string        `yaml:"username,omitempty"`
	Password     string        `yaml:"password,omitempty"`
	RelayThrough *RelayThrough `yaml:"relay_through,omitempty"`
}

// RelayThrough defines how a custom proxy chains through upstream nodes.
type RelayThrough struct {
	Type     string `yaml:"type"`           // "group" | "select" | "all"
	Strategy string `yaml:"strategy"`       // "select" | "url-test"
	Name     string `yaml:"name,omitempty"` // required when Type=group
	Match    string `yaml:"match,omitempty"` // required when Type=select
}

// Group defines a region node group.
type Group struct {
	Match    string `yaml:"match"`
	Strategy string `yaml:"strategy"` // "select" | "url-test"
}

// Filters defines subscription node filtering rules.
type Filters struct {
	Exclude string `yaml:"exclude,omitempty"`
}
