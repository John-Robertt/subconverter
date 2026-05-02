package server

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/John-Robertt/subconverter/internal/generate"
)

const maxFilenameLength = 255

// handleGenerate executes the pipeline and renders output in the requested format.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	format := query.Get("format")
	if format != "clash" && format != "surge" {
		writeError(w, http.StatusBadRequest, "format 参数无效：必须为 clash 或 surge")
		return
	}

	if !s.isAuthorized(r, query.Get("token")) {
		writeError(w, http.StatusUnauthorized, "访问令牌缺失或无效")
		return
	}

	filename, err := resolveFilename(query, format)
	if err != nil {
		code, msg := presentError(err)
		writeError(w, code, msg)
		return
	}

	result, err := s.generator.Generate(r.Context(), generate.Request{
		Format:   format,
		Filename: filename,
	})
	if err != nil {
		code, msg := presentError(err)
		log.Printf("generate error: %v", err)
		writeError(w, code, msg)
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", contentDispositionValue(result.Filename))
	_, _ = w.Write(result.Body)
}

// handleHealthz returns 200 OK for health checks.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) isAuthorized(r *http.Request, providedToken string) bool {
	if s.opts.AdminSessionValidator != nil && s.opts.AdminSessionValidator(r) {
		return true
	}
	if s.opts.AccessToken == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(s.opts.AccessToken)) == 1
}

func resolveFilename(query url.Values, format string) (string, error) {
	raw := defaultFilename(format)
	if _, ok := query["filename"]; ok {
		raw = query.Get("filename")
		if raw == "" {
			return "", badRequest("filename 参数无效：不能为空")
		}
	}

	if len(raw) > maxFilenameLength {
		return "", badRequest("filename 参数无效：长度不能超过 255 个字符")
	}
	if err := validateFilename(raw); err != nil {
		return "", err
	}

	ext := expectedExtension(format)
	name := raw
	currentExt := path.Ext(name)
	if currentExt == "" {
		name += ext
		currentExt = ext
	}
	if !strings.EqualFold(currentExt, ext) {
		switch format {
		case "clash":
			return "", badRequest("filename 参数无效：Clash 配置必须使用 .yaml 扩展名")
		case "surge":
			return "", badRequest("filename 参数无效：Surge 配置必须使用 .conf 扩展名")
		default:
			return "", badRequest("filename 参数无效：扩展名不正确")
		}
	}
	if base := strings.TrimSuffix(name, currentExt); strings.Trim(base, ".") == "" {
		return "", badRequest("filename 参数无效：文件名主体不能为空")
	}
	return name, nil
}

func validateFilename(name string) error {
	for _, r := range name {
		switch {
		case r > 127:
			return badRequest("filename 参数无效：仅允许 ASCII 字母、数字、点号(.)、连字符(-)、下划线(_)")
		case r < 32 || r == 127:
			return badRequest("filename 参数无效：不能包含控制字符")
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_':
		default:
			return badRequest("filename 参数无效：仅允许 ASCII 字母、数字、点号(.)、连字符(-)、下划线(_)")
		}
	}
	return nil
}

func defaultFilename(format string) string {
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

func contentDispositionValue(filename string) string {
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		filename,
		url.PathEscape(filename),
	)
}
