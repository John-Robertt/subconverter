package proxyparse

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/ssparse"
)

// PluginSpec is a protocol-agnostic plugin declaration attached to a parsed
// proxy URL.
type PluginSpec struct {
	Name string
	Opts map[string]string
}

// Result is the normalized parse result for a custom proxy URL.
type Result struct {
	Type   string
	Server string
	Port   int
	Params map[string]string
	Plugin *PluginSpec
}

// ParseURL parses a custom proxy URL into a runtime-neutral structure.
// Supported schemes: ss://, socks5://, http://.
func ParseURL(rawURL string) (Result, error) {
	switch {
	case strings.HasPrefix(rawURL, "ss://"):
		return parseSSProxyURL(rawURL)
	case strings.HasPrefix(rawURL, "socks5://"):
		return parsePlainProxyURL(rawURL, "socks5")
	case strings.HasPrefix(rawURL, "http://"):
		return parsePlainProxyURL(rawURL, "http")
	default:
		return Result{}, fmt.Errorf("不支持的协议，支持 ss://、socks5://、http://")
	}
}

func parsePlainProxyURL(rawURL, proxyType string) (Result, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Result{}, fmt.Errorf("URL 格式无效：%v", err)
	}
	if u.Path != "" {
		return Result{}, fmt.Errorf("不支持 path，格式必须为 %s://[user:pass@]server:port", proxyType)
	}
	if u.RawQuery != "" {
		return Result{}, fmt.Errorf("不支持 query 参数，格式必须为 %s://[user:pass@]server:port", proxyType)
	}
	if u.Fragment != "" {
		return Result{}, fmt.Errorf("不支持 fragment，格式必须为 %s://[user:pass@]server:port", proxyType)
	}

	host := u.Hostname()
	if host == "" {
		return Result{}, fmt.Errorf("缺少服务器地址")
	}

	portStr := u.Port()
	if portStr == "" {
		return Result{}, fmt.Errorf("缺少端口")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Result{}, fmt.Errorf("端口 %q 不是合法数字", portStr)
	}
	if port < 1 || port > 65535 {
		return Result{}, fmt.Errorf("端口 %d 超出 1-65535 范围", port)
	}

	params := make(map[string]string)
	if u.User != nil {
		if username := u.User.Username(); username != "" {
			params["username"] = username
		}
		if password, ok := u.User.Password(); ok && password != "" {
			params["password"] = password
		}
	}

	return Result{
		Type:   proxyType,
		Server: host,
		Port:   port,
		Params: params,
	}, nil
}

func parseSSProxyURL(rawURL string) (Result, error) {
	r, err := ssparse.ParseBody(rawURL[len("ss://"):], false)
	if err != nil {
		return Result{}, fmt.Errorf("SS URI 解析失败：%v", err)
	}

	return Result{
		Type:   "ss",
		Server: r.Server,
		Port:   r.Port,
		Params: map[string]string{
			"cipher":   r.Cipher,
			"password": r.Password,
		},
		Plugin: copyPlugin(r.Plugin),
	}, nil
}

func copyPlugin(src *ssparse.PluginSpec) *PluginSpec {
	if src == nil {
		return nil
	}
	dst := &PluginSpec{Name: src.Name}
	if len(src.Opts) > 0 {
		dst.Opts = make(map[string]string, len(src.Opts))
		for k, v := range src.Opts {
			dst.Opts[k] = v
		}
	}
	return dst
}
