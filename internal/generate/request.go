package generate

import (
	"errors"
	"net/url"
	"path"
	"strings"
)

const MaxFilenameLength = 255

// ValidFormat reports whether format is one of the supported generation
// targets.
func ValidFormat(format string) bool {
	return format == "clash" || format == "surge"
}

// ResolveFilename normalizes the optional filename query parameter.
func ResolveFilename(raw string, present bool, format string) (string, error) {
	name := DefaultFilename(format)
	if present {
		name = raw
		if name == "" {
			return "", errors.New("filename 参数无效：不能为空")
		}
	}

	if len(name) > MaxFilenameLength {
		return "", errors.New("filename 参数无效：长度不能超过 255 个字符")
	}
	if err := ValidateFilename(name); err != nil {
		return "", err
	}

	ext := expectedExtension(format)
	currentExt := path.Ext(name)
	if currentExt == "" {
		name += ext
		currentExt = ext
	}
	if !strings.EqualFold(currentExt, ext) {
		switch format {
		case "clash":
			return "", errors.New("filename 参数无效：Clash 配置必须使用 .yaml 扩展名")
		case "surge":
			return "", errors.New("filename 参数无效：Surge 配置必须使用 .conf 扩展名")
		default:
			return "", errors.New("filename 参数无效：扩展名不正确")
		}
	}
	if base := strings.TrimSuffix(name, currentExt); strings.Trim(base, ".") == "" {
		return "", errors.New("filename 参数无效：文件名主体不能为空")
	}
	return name, nil
}

// ValidateFilename enforces the filename character set shared by /generate,
// preview, and server-generated subscription links.
func ValidateFilename(name string) error {
	for _, r := range name {
		switch {
		case r > 127:
			return errors.New("filename 参数无效：仅允许 ASCII 字母、数字、点号(.)、连字符(-)、下划线(_)")
		case r < 32 || r == 127:
			return errors.New("filename 参数无效：不能包含控制字符")
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_':
		default:
			return errors.New("filename 参数无效：仅允许 ASCII 字母、数字、点号(.)、连字符(-)、下划线(_)")
		}
	}
	return nil
}

func DefaultFilename(format string) string {
	switch format {
	case "clash":
		return "clash.yaml"
	case "surge":
		return "surge.conf"
	default:
		return "download"
	}
}

func expectedExtension(format string) string {
	switch format {
	case "clash":
		return ".yaml"
	case "surge":
		return ".conf"
	default:
		return ""
	}
}

// BuildGenerateURL builds a client-facing /generate URL from a validated
// base_url and normalized filename.
func BuildGenerateURL(baseURL, format, filename, accessToken string, includeToken bool) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	base.Path = "/generate"
	base.RawPath = ""

	params := []string{"format=" + url.QueryEscape(format)}
	if includeToken && accessToken != "" {
		params = append(params, "token="+url.QueryEscape(accessToken))
	}
	params = append(params, "filename="+url.QueryEscape(filename))
	base.RawQuery = strings.Join(params, "&")
	return base.String(), nil
}
