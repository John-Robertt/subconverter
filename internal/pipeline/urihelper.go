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
			return "", "", fmt.Errorf("missing port")
		}
		return "", "", fmt.Errorf("invalid host:port %q", hostport)
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
	return "", fmt.Errorf("not valid base64")
}
