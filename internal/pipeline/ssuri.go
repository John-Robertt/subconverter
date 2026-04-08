package pipeline

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ParseSSURI parses a single SS URI into a model.Proxy.
//
// Expected format: ss://BASE64(method:password)@server:port#NodeName
//
// The userinfo part (method:password) is base64-encoded and may or may not
// include padding. The fragment (#NodeName) is URL-encoded.
func ParseSSURI(raw string) (model.Proxy, error) {
	const prefix = "ss://"
	if !strings.HasPrefix(raw, prefix) {
		return model.Proxy{}, ssError(raw, "missing ss:// prefix")
	}

	body := raw[len(prefix):]

	// Split fragment (node name).
	name := ""
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		fragment := body[idx+1:]
		body = body[:idx]
		decoded, err := url.PathUnescape(fragment)
		if err != nil {
			return model.Proxy{}, ssError(raw, fmt.Sprintf("invalid fragment encoding: %v", err))
		}
		name = decoded
	}
	if name == "" {
		return model.Proxy{}, ssError(raw, "missing or empty node name")
	}

	// Split userinfo and host:port on the last '@'.
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return model.Proxy{}, ssError(raw, "missing @ separator")
	}
	userinfo := body[:atIdx]
	hostport := body[atIdx+1:]

	// Decode base64 userinfo (method:password).
	decoded, err := decodeBase64(userinfo)
	if err != nil {
		return model.Proxy{}, ssError(raw, fmt.Sprintf("invalid base64 userinfo: %v", err))
	}

	parts := strings.SplitN(decoded, ":", 2)
	if len(parts) != 2 {
		return model.Proxy{}, ssError(raw, "userinfo missing ':' separator between method and password")
	}
	method := parts[0]
	password := parts[1]
	if method == "" {
		return model.Proxy{}, ssError(raw, "empty cipher method")
	}

	// Parse host:port.
	colonIdx := strings.LastIndex(hostport, ":")
	if colonIdx < 0 {
		return model.Proxy{}, ssError(raw, "missing port")
	}
	server := hostport[:colonIdx]
	portStr := hostport[colonIdx+1:]
	if server == "" {
		return model.Proxy{}, ssError(raw, "empty server host")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return model.Proxy{}, ssError(raw, fmt.Sprintf("invalid port %q", portStr))
	}
	if port < 1 || port > 65535 {
		return model.Proxy{}, ssError(raw, fmt.Sprintf("port %d out of range 1-65535", port))
	}

	return model.Proxy{
		Name:   name,
		Type:   "ss",
		Server: server,
		Port:   port,
		Params: map[string]string{
			"cipher":   method,
			"password": password,
		},
		Kind: model.KindSubscription,
	}, nil
}

// decodeBase64 tries multiple base64 encodings to handle both
// padded and unpadded variants commonly found in SS subscriptions.
func decodeBase64(s string) (string, error) {
	// Try standard encoding with padding first.
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	// Try without padding (common in SS URIs).
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	// Try URL-safe variants.
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	return "", fmt.Errorf("not valid base64")
}

func ssError(uri, reason string) error {
	// Truncate URI in error messages to avoid leaking full credentials.
	display := uri
	if len(display) > 80 {
		display = display[:77] + "..."
	}
	return &errtype.BuildError{
		Phase:   "source",
		Message: fmt.Sprintf("invalid SS URI %q: %s", display, reason),
	}
}
