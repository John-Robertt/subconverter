package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/ssparse"
)

type proxyURLResult struct {
	Type   string
	Server string
	Port   int
	Params map[string]string
	Plugin *model.Plugin
}

func parseProxyURL(rawURL string) (proxyURLResult, error) {
	switch {
	case strings.HasPrefix(rawURL, "ss://"):
		return parseSSProxyURL(rawURL)
	case strings.HasPrefix(rawURL, "socks5://"):
		return parsePlainProxyURL(rawURL, "socks5")
	case strings.HasPrefix(rawURL, "http://"):
		return parsePlainProxyURL(rawURL, "http")
	default:
		return proxyURLResult{}, fmt.Errorf("不支持的协议，支持 ss://、socks5://、http://")
	}
}

func parsePlainProxyURL(rawURL, proxyType string) (proxyURLResult, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return proxyURLResult{}, fmt.Errorf("URL 格式无效：%v", err)
	}
	if u.Path != "" {
		return proxyURLResult{}, fmt.Errorf("不支持 path，格式必须为 %s://[user:pass@]server:port", proxyType)
	}
	if u.RawQuery != "" {
		return proxyURLResult{}, fmt.Errorf("不支持 query 参数，格式必须为 %s://[user:pass@]server:port", proxyType)
	}
	if u.Fragment != "" {
		return proxyURLResult{}, fmt.Errorf("不支持 fragment，格式必须为 %s://[user:pass@]server:port", proxyType)
	}

	host := u.Hostname()
	if host == "" {
		return proxyURLResult{}, fmt.Errorf("缺少服务器地址")
	}

	portStr := u.Port()
	if portStr == "" {
		return proxyURLResult{}, fmt.Errorf("缺少端口")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return proxyURLResult{}, fmt.Errorf("端口 %q 不是合法数字", portStr)
	}
	if port < 1 || port > 65535 {
		return proxyURLResult{}, fmt.Errorf("端口 %d 超出 1-65535 范围", port)
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

	return proxyURLResult{
		Type:   proxyType,
		Server: host,
		Port:   port,
		Params: params,
	}, nil
}

// parseSSProxyURL delegates SS URI parsing to the shared ssparse package.
// The fragment (#NodeName) is silently ignored; the caller supplies the name.
func parseSSProxyURL(rawURL string) (proxyURLResult, error) {
	r, err := ssparse.ParseBody(rawURL[len("ss://"):], false)
	if err != nil {
		return proxyURLResult{}, fmt.Errorf("SS URI 解析失败：%v", err)
	}

	return proxyURLResult{
		Type:   "ss",
		Server: r.Server,
		Port:   r.Port,
		Params: map[string]string{
			"cipher":   r.Cipher,
			"password": r.Password,
		},
		Plugin: r.Plugin,
	}, nil
}
