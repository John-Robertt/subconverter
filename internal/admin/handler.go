package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/John-Robertt/subconverter/internal/app"
	"github.com/John-Robertt/subconverter/internal/auth"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/generate"
)

type Handler struct {
	app  *app.Service
	auth *auth.Service
}

func New(appSvc *app.Service, authSvc *auth.Service) *Handler {
	return &Handler{app: appSvc, auth: authSvc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	if !isSafeMethod(r.Method) && !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "csrf_check_failed", "请求来源无效")
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/auth/status":
		h.handleAuthStatus(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/auth/login":
		h.handleAuthLogin(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/auth/setup":
		h.handleAuthSetup(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/auth/logout":
		h.handleAuthLogout(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/config":
		if !h.requireSession(w, r) {
			return
		}
		h.handleConfigGet(w, r)
	case r.Method == http.MethodPut && r.URL.Path == "/api/config":
		if !h.requireSession(w, r) {
			return
		}
		h.handleConfigPut(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/config/validate":
		if !h.requireSession(w, r) {
			return
		}
		h.handleConfigValidate(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/reload":
		if !h.requireSession(w, r) {
			return
		}
		h.handleReload(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/preview/nodes":
		if !h.requireSession(w, r) {
			return
		}
		h.handlePreviewNodesGet(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/preview/nodes":
		if !h.requireSession(w, r) {
			return
		}
		h.handlePreviewNodesPost(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/preview/groups":
		if !h.requireSession(w, r) {
			return
		}
		h.handlePreviewGroupsGet(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/preview/groups":
		if !h.requireSession(w, r) {
			return
		}
		h.handlePreviewGroupsPost(w, r)
	case (r.Method == http.MethodGet || r.Method == http.MethodPost) && r.URL.Path == "/api/generate/preview":
		if !h.requireSession(w, r) {
			return
		}
		h.handleGeneratePreview(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/generate/link":
		if !h.requireSession(w, r) {
			return
		}
		h.handleGenerateLink(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/status":
		if !h.requireSession(w, r) {
			return
		}
		h.handleStatus(w, r)
	default:
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
	}
}

func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.auth.Status(sessionID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_state_error", "读取认证状态失败")
		return
	}
	body := struct {
		Authed             bool   `json:"authed"`
		SetupRequired      bool   `json:"setup_required"`
		SetupTokenRequired bool   `json:"setup_token_required"`
		LockedUntil        string `json:"locked_until"`
	}{
		Authed:             status.Authed,
		SetupRequired:      status.SetupRequired,
		SetupTokenRequired: status.SetupTokenRequired,
		LockedUntil:        formatTime(status.LockedUntil),
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *Handler) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var input auth.LoginInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Username == "" || input.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "用户名和密码必填")
		return
	}
	session, err := h.auth.Login(r.RemoteAddr, input)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	setSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, map[string]string{"redirect": "/sources"})
}

func (h *Handler) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	var input auth.SetupInput
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := h.auth.Setup(input)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	setSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, map[string]string{"redirect": "/sources"})
}

func (h *Handler) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.Logout(sessionID(r)); err != nil {
		writeError(w, http.StatusInternalServerError, "auth_state_error", "注销失败")
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.ConfigSnapshot(r.Context())
	if err != nil {
		writeServiceError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	var input app.SaveConfigInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if len(input.Config) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "缺少 config")
		return
	}
	result, err := h.app.SaveConfig(r.Context(), &input)
	if err != nil {
		writeServiceError(w, err, true)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleConfigValidate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Config json.RawMessage `json:"config"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if len(input.Config) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "缺少 config")
		return
	}
	result, err := h.app.ValidateDraft(r.Context(), input.Config)
	if err != nil {
		writeServiceError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleReload(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.Reload(r.Context())
	if err != nil {
		writeServiceError(w, err, true)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePreviewNodesGet(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.PreviewNodes(r.Context())
	if err != nil {
		writeServiceError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePreviewNodesPost(w http.ResponseWriter, r *http.Request) {
	configJSON, ok := decodeConfigBody(w, r)
	if !ok {
		return
	}
	result, err := h.app.PreviewNodesFromDraft(r.Context(), configJSON)
	if err != nil {
		writeServiceError(w, err, true)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePreviewGroupsGet(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.PreviewGroups(r.Context())
	if err != nil {
		writeServiceError(w, err, true)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePreviewGroupsPost(w http.ResponseWriter, r *http.Request) {
	configJSON, ok := decodeConfigBody(w, r)
	if !ok {
		return
	}
	result, err := h.app.PreviewGroupsFromDraft(r.Context(), configJSON)
	if err != nil {
		writeServiceError(w, err, true)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGeneratePreview(w http.ResponseWriter, r *http.Request) {
	req, ok := parseGenerateRequest(w, r)
	if !ok {
		return
	}

	var (
		result *generate.Result
		err    error
	)
	if r.Method == http.MethodPost {
		configJSON, bodyOK := decodeConfigBody(w, r)
		if !bodyOK {
			return
		}
		result, err = h.app.GenerateFromDraft(r.Context(), req, configJSON)
	} else {
		result, err = h.app.Generate(r.Context(), req)
	}
	if err != nil {
		writeServiceError(w, err, true)
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Body)
}

func (h *Handler) handleGenerateLink(w http.ResponseWriter, r *http.Request) {
	req, ok := parseGenerateRequest(w, r)
	if !ok {
		return
	}
	includeToken := true
	if raw := r.URL.Query().Get("include_token"); raw != "" {
		switch raw {
		case "true":
			includeToken = true
		case "false":
			includeToken = false
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", "include_token 参数无效")
			return
		}
	}
	result, err := h.app.GenerateLink(r.Context(), &app.GenerateLinkInput{
		Format:       req.Format,
		Filename:     req.Filename,
		IncludeToken: includeToken,
	})
	if err != nil {
		writeServiceError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.Status(r.Context())
	if err != nil {
		writeServiceError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) requireSession(w http.ResponseWriter, r *http.Request) bool {
	id := sessionID(r)
	if id == "" {
		writeError(w, http.StatusUnauthorized, auth.CodeAuthRequired, "缺少管理员 session")
		return false
	}
	if err := h.auth.ValidateSession(id); err != nil {
		h.writeAuthError(w, err)
		return false
	}
	return true
}

func (h *Handler) writeAuthError(w http.ResponseWriter, err error) {
	var authErr *auth.Error
	if !errors.As(err, &authErr) {
		writeError(w, http.StatusInternalServerError, "auth_error", "认证失败")
		return
	}
	status := http.StatusBadRequest
	switch authErr.Code {
	case auth.CodeInvalidCredentials, auth.CodeSessionExpired, auth.CodeAuthRequired, auth.CodeSetupTokenRequired, auth.CodeSetupTokenInvalid:
		status = http.StatusUnauthorized
	case auth.CodeAuthLocked:
		status = http.StatusLocked
	case auth.CodeSetupNotAllowed, auth.CodeAuthStateNotWritable:
		status = http.StatusConflict
	}
	extra := map[string]any{}
	if authErr.Remaining > 0 {
		extra["remaining"] = authErr.Remaining
	}
	if !authErr.Until.IsZero() {
		extra["until"] = authErr.Until.Format(time.RFC3339)
	}
	writeErrorWithExtra(w, status, authErr.Code, authErr.Message, extra)
}

func writeServiceError(w http.ResponseWriter, err error, validationBody bool) {
	if validationBody {
		if result, ok := app.ValidateResultFromError(err); ok {
			writeJSON(w, http.StatusBadRequest, result)
			return
		}
		if result, ok := app.GraphValidateResultFromError(err); ok {
			writeJSON(w, http.StatusBadRequest, result)
			return
		}
	}

	var badReq *app.BadRequestError
	if errors.As(err, &badReq) {
		writeError(w, http.StatusBadRequest, badReq.Code, badReq.Message)
		return
	}

	var revisionErr *errtype.RevisionConflictError
	if errors.As(err, &revisionErr) {
		writeErrorWithExtra(w, http.StatusConflict, "config_revision_conflict", "配置文件已被其他来源修改，请重新读取后再保存", map[string]any{
			"current_config_revision": revisionErr.CurrentConfigRevision,
		})
		return
	}
	if errors.Is(err, errtype.ErrConfigSourceReadonly) {
		writeError(w, http.StatusConflict, "config_source_readonly", "当前配置源只读")
		return
	}
	if errors.Is(err, errtype.ErrConfigFileNotWritable) {
		writeError(w, http.StatusConflict, "config_file_not_writable", "配置文件不可写")
		return
	}
	if errors.Is(err, errtype.ErrReloadInProgress) {
		writeError(w, http.StatusTooManyRequests, "reload_in_progress", "已有 reload 正在执行")
		return
	}

	var fetchErr *errtype.FetchError
	if errors.As(err, &fetchErr) {
		writeError(w, http.StatusBadGateway, string(fetchErr.Code), fetchErr.Message)
		return
	}
	var cfgErr *errtype.ConfigError
	if errors.As(err, &cfgErr) {
		writeError(w, http.StatusBadRequest, string(cfgErr.Code), cfgErr.Message)
		return
	}
	var buildErr *errtype.BuildError
	if errors.As(err, &buildErr) {
		writeError(w, http.StatusBadRequest, string(buildErr.Code), buildErr.Message)
		return
	}
	var targetErr *errtype.TargetError
	if errors.As(err, &targetErr) {
		status := http.StatusInternalServerError
		if isUserFixableTargetError(targetErr) {
			status = http.StatusBadRequest
		}
		writeError(w, status, string(targetErr.Code), targetErr.Message)
		return
	}
	var resourceErr *errtype.ResourceError
	if errors.As(err, &resourceErr) {
		writeError(w, http.StatusInternalServerError, string(resourceErr.Code), resourceErr.Message)
		return
	}
	var renderErr *errtype.RenderError
	if errors.As(err, &renderErr) {
		writeError(w, http.StatusInternalServerError, string(renderErr.Code), renderErr.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", "内部错误")
}

func isUserFixableTargetError(err *errtype.TargetError) bool {
	return err.Code == errtype.CodeTargetClashFallbackEmpty || err.Code == errtype.CodeTargetSurgeFallbackEmpty
}

func decodeConfigBody(w http.ResponseWriter, r *http.Request) (json.RawMessage, bool) {
	var input struct {
		Config json.RawMessage `json:"config"`
	}
	if !decodeJSON(w, r, &input) {
		return nil, false
	}
	if len(input.Config) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "缺少 config")
		return nil, false
	}
	return input.Config, true
}

func parseGenerateRequest(w http.ResponseWriter, r *http.Request) (generate.Request, bool) {
	query := r.URL.Query()
	format := query.Get("format")
	if !generate.ValidFormat(format) {
		writeError(w, http.StatusBadRequest, "invalid_request", "format 参数无效：必须为 clash 或 surge")
		return generate.Request{}, false
	}
	_, filenamePresent := query["filename"]
	filename, err := generate.ResolveFilename(query.Get("filename"), filenamePresent, format)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return generate.Request{}, false
	}
	return generate.Request{Format: format, Filename: filename}, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求 JSON 无法解析")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeErrorWithExtra(w, status, code, message, nil)
}

func writeErrorWithExtra(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	errBody := map[string]any{
		"code":    code,
		"message": message,
	}
	for key, value := range extra {
		errBody[key] = value
	}
	writeJSON(w, status, map[string]any{"error": errBody})
}

func sessionID(r *http.Request) string {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, session *auth.Session) {
	cookie := &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
	}
	if session.Remember {
		cookie.Expires = session.ExpiresAt
		cookie.MaxAge = int(time.Until(session.ExpiresAt).Seconds())
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
		MaxAge:   -1,
	})
}

func sameOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		return originMatchesHost(origin, r.Host)
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		return originMatchesHost(referer, r.Host)
	}
	return false
}

func originMatchesHost(raw, host string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
