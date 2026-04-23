package pipeline

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"
)

func splitHostPort(hostport string) (string, string, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return "", "", fmt.Errorf("缺少端口")
		}
		return "", "", fmt.Errorf("host:port 无效 %q", hostport)
	}
	return host, port, nil
}

func decodeBase64(s string) (string, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	return "", fmt.Errorf("base64 解码失败")
}
