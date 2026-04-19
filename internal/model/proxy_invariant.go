package model

import "fmt"

var allowedProxyTypesByKind = map[ProxyKind]map[string]bool{
	KindSubscription: {
		"ss": true,
	},
	KindSnell: {
		"snell": true,
	},
	KindVLess: {
		"vless": true,
	},
	KindCustom: {
		"ss":     true,
		"socks5": true,
		"http":   true,
	},
	KindChained: {
		"ss":     true,
		"socks5": true,
		"http":   true,
	},
}

// ValidateProxyInvariant validates the structural invariants of one IR proxy.
// It centralizes the meaning of Kind/Type/Params/Dialer so downstream stages
// don't each need to re-interpret partially overlapping rules.
func ValidateProxyInvariant(p Proxy) error {
	if p.Name == "" {
		return fmt.Errorf("name 不能为空")
	}
	if p.Server == "" {
		return fmt.Errorf("server 不能为空")
	}
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("port %d 超出 1-65535 范围", p.Port)
	}
	if p.Kind == "" {
		return fmt.Errorf("kind 不能为空")
	}
	if p.Type == "" {
		return fmt.Errorf("type 不能为空")
	}

	allowedTypes, ok := allowedProxyTypesByKind[p.Kind]
	if !ok {
		return fmt.Errorf("kind %q 不受支持", p.Kind)
	}
	if !allowedTypes[p.Type] {
		return fmt.Errorf("kind=%q 不能使用 type=%q", p.Kind, p.Type)
	}

	if p.Kind == KindChained {
		if p.Dialer == "" {
			return fmt.Errorf("kind=chained 时 dialer 必填")
		}
	} else if p.Dialer != "" {
		return fmt.Errorf("仅 kind=chained 允许设置 dialer")
	}

	for _, key := range requiredProxyParams(p.Type) {
		if p.Params[key] == "" {
			return fmt.Errorf("type=%s 缺少必填参数 %s", p.Type, key)
		}
	}

	return nil
}

func requiredProxyParams(proxyType string) []string {
	switch proxyType {
	case "ss":
		return []string{"cipher", "password"}
	case "snell":
		return []string{"psk"}
	case "vless":
		return []string{"uuid", "network"}
	default:
		return nil
	}
}
