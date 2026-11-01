package pipeline

import (
	"strings"

	"github.com/John-Robertt/subconverter/internal/model"
)

// SubscriptionInspection describes parsed nodes without applying global filters or groups.
type SubscriptionInspection struct {
	Format     string
	Candidates []string
	Proxies    []model.Proxy
}

// InspectSubscription prefers document evidence and asks the caller to resolve ambiguous INI syntax.
// The legacy Source auto-selection order is intentionally unchanged.
func InspectSubscription(location string, body []byte, format string, allowFragment bool) (*SubscriptionInspection, error) {
	if format == "" || format == "auto" {
		if document, _ := parseClashSubscriptionDocument(body, false); document != nil {
			format = "clash"
		} else {
			if !hasINISection(string(body)) {
				if proxies, _, err := parseSubscriptionBody(location, body, "text", allowFragment); err == nil {
					return &SubscriptionInspection{Format: "text", Proxies: assignProxyIDs(proxies)}, nil
				}
			}
			format = inspectINIFormat(location, string(body), allowFragment)
			if format == "" {
				proxies, _, err := parseSubscriptionBody(location, body, "text", allowFragment)
				if err != nil {
					return nil, err
				}
				return &SubscriptionInspection{Format: "text", Proxies: assignProxyIDs(proxies)}, nil
			}
		}
	}
	if format == "ambiguous" {
		proxies, _, err := parseSubscriptionBody(location, body, "surge", allowFragment)
		if err != nil {
			return nil, err
		}
		return &SubscriptionInspection{Candidates: []string{"surge", "loon"}, Proxies: assignProxyIDs(proxies)}, nil
	}
	proxies, selected, err := parseSubscriptionBody(location, body, format, allowFragment)
	if err != nil {
		return nil, err
	}
	return &SubscriptionInspection{Format: selected, Proxies: assignProxyIDs(proxies)}, nil
}

func inspectINIFormat(location, body string, allowFragment bool) string {
	loon, surge, statements := false, false, false
	inProxy := allowFragment && !hasINISection(body)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimPrefix(strings.TrimSpace(raw), "\ufeff")
		if isINISectionHeader(line) {
			header := strings.ToLower(line)
			inProxy = header == "[proxy]"
			switch header {
			case "[remote proxy]", "[remote filter]", "[remote rule]", "[proxy chain]":
				loon = true
			}
			continue
		}
		if !inProxy || isSubscriptionDocumentComment(line) {
			continue
		}
		_, payload, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		statements = true
		parts := strings.SplitN(strings.TrimSpace(payload), ",", 4)
		switch strings.ToLower(strings.TrimSpace(parts[0])) {
		case "shadowsocks", "shadowsocksr":
			loon = true
		case "ss", "snell":
			surge = true
		default:
			if len(parts) < 4 {
				continue
			}
			credentials := strings.TrimSpace(parts[3])
			if strings.HasPrefix(credentials, "\"") || !strings.Contains(strings.SplitN(credentials, ",", 2)[0], "=") {
				loon = true
			} else if strings.HasPrefix(credentials, "password=") || strings.HasPrefix(credentials, "username=") {
				surge = true
			}
		}
	}
	if loon && !surge {
		return "loon"
	}
	if surge && !loon {
		return "surge"
	}
	if !loon && !surge && subscriptionDocumentFormatFromURL(location) == model.NativeProxyFormatLoon {
		return "loon"
	}
	if statements || loon || surge {
		return "ambiguous"
	}
	return ""
}
