package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	SessionCookieName = "session_id"

	passwordAlgorithm  = "pbkdf2-sha256" // #nosec G101 -- algorithm identifier, not a hardcoded credential.
	passwordIterations = 600000
	passwordSaltBytes  = 32
	passwordKeyBytes   = 32

	sessionBytes       = 32
	normalSessionTTL   = 24 * time.Hour
	rememberSessionTTL = 7 * 24 * time.Hour
	maxFailures        = 5
	lockDuration       = 15 * time.Minute
)

type Options struct {
	StatePath  string
	SetupToken string
	Logger     *log.Logger
	Now        func() time.Time
}

type Service struct {
	statePath  string
	setupToken string
	logger     *log.Logger
	now        func() time.Time
	mu         sync.Mutex
}

type Status struct {
	Authed             bool      `json:"authed"`
	SetupRequired      bool      `json:"setup_required"`
	SetupTokenRequired bool      `json:"setup_token_required"`
	LockedUntil        time.Time `json:"-"`
}

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

type SetupInput struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	SetupToken string `json:"setup_token"`
}

type Session struct {
	ID        string
	ExpiresAt time.Time
	Remember  bool
}

type stateFile struct {
	Username     string                   `json:"username,omitempty"`
	PasswordHash string                   `json:"password_hash,omitempty"`
	Sessions     map[string]sessionRecord `json:"sessions,omitempty"`
	Failures     map[string]failureRecord `json:"failures,omitempty"`
}

type sessionRecord struct {
	ExpiresAt time.Time `json:"expires_at"`
}

type failureRecord struct {
	Count       int       `json:"count"`
	LockedUntil time.Time `json:"locked_until,omitempty"`
}

func New(opts Options) (*Service, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}
	s := &Service{
		statePath:  opts.StatePath,
		setupToken: opts.SetupToken,
		logger:     logger,
		now:        now,
	}

	state, err := s.loadState()
	if err != nil {
		return nil, err
	}
	if !state.hasAdmin() && s.setupToken == "" {
		token, err := randomToken(32)
		if err != nil {
			return nil, err
		}
		s.setupToken = token
		s.logger.Printf("subconverter setup token: %s", token)
	}
	return s, nil
}

func (s *Service) Status(sessionID string) (Status, error) {
	s.lock()
	defer s.unlock()

	state, err := s.loadState()
	if err != nil {
		return Status{}, err
	}
	status := Status{
		Authed:             s.validateSessionLocked(state, sessionID) == nil,
		SetupRequired:      !state.hasAdmin(),
		SetupTokenRequired: !state.hasAdmin(),
		LockedUntil:        state.latestLockedUntil(s.now()),
	}
	return status, nil
}

func (s *Service) Login(remoteAddr string, input LoginInput) (*Session, error) {
	s.lock()
	defer s.unlock()

	state, err := s.loadState()
	if err != nil {
		return nil, err
	}
	key := failureKey(remoteAddr, input.Username)
	if failure := state.failure(key); failure.LockedUntil.After(s.now()) {
		return nil, &Error{Code: CodeAuthLocked, Message: "登录失败次数过多，请稍后再试", Until: failure.LockedUntil}
	}
	if !state.hasAdmin() || input.Username != state.Username || !verifyPassword(input.Password, state.PasswordHash) {
		return nil, s.recordFailure(&state, key)
	}

	if needsPasswordRehash(state.PasswordHash) {
		hash, err := hashPassword(input.Password)
		if err != nil {
			return nil, err
		}
		state.PasswordHash = hash
	}
	delete(state.Failures, key)
	session, err := s.createSessionLocked(&state, input.Remember)
	if err != nil {
		return nil, err
	}
	if err := s.writeState(state); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Service) Setup(input SetupInput) (*Session, error) {
	s.lock()
	defer s.unlock()

	state, err := s.loadState()
	if err != nil {
		return nil, err
	}
	if state.hasAdmin() {
		return nil, authError(CodeSetupNotAllowed, "管理员凭据已存在")
	}
	if s.setupToken != "" && input.SetupToken == "" {
		return nil, authError(CodeSetupTokenRequired, "缺少 setup token")
	}
	if s.setupToken != "" && subtle.ConstantTimeCompare([]byte(input.SetupToken), []byte(s.setupToken)) != 1 {
		return nil, authError(CodeSetupTokenInvalid, "setup token 无效")
	}
	if input.Username == "" || input.Password == "" {
		return nil, authError("invalid_request", "用户名和密码必填")
	}
	if len(input.Password) < 12 {
		return nil, authError(CodePasswordTooWeak, "密码至少需要 12 位")
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	state.Username = input.Username
	state.PasswordHash = passwordHash
	state.Sessions = map[string]sessionRecord{}
	state.Failures = map[string]failureRecord{}
	session, err := s.createSessionLocked(&state, false)
	if err != nil {
		return nil, err
	}
	if err := s.writeState(state); err != nil {
		return nil, authError(CodeAuthStateNotWritable, "auth state 不可写")
	}
	return session, nil
}

func (s *Service) Logout(sessionID string) error {
	if sessionID == "" {
		return nil
	}
	s.lock()
	defer s.unlock()

	state, err := s.loadState()
	if err != nil {
		return err
	}
	delete(state.Sessions, hashSessionID(sessionID))
	return s.writeState(state)
}

func (s *Service) ValidateSession(sessionID string) error {
	if sessionID == "" {
		return authError(CodeAuthRequired, "缺少管理员 session")
	}
	s.lock()
	defer s.unlock()

	state, err := s.loadState()
	if err != nil {
		return err
	}
	return s.validateSessionLocked(state, sessionID)
}

func (s *Service) IsSessionValid(sessionID string) bool {
	return s.ValidateSession(sessionID) == nil
}

func (s *Service) createSessionLocked(state *stateFile, remember bool) (*Session, error) {
	id, err := randomToken(sessionBytes)
	if err != nil {
		return nil, err
	}
	ttl := normalSessionTTL
	if remember {
		ttl = rememberSessionTTL
	}
	expiresAt := s.now().Add(ttl)
	if state.Sessions == nil {
		state.Sessions = map[string]sessionRecord{}
	}
	state.Sessions[hashSessionID(id)] = sessionRecord{ExpiresAt: expiresAt}
	return &Session{ID: id, ExpiresAt: expiresAt, Remember: remember}, nil
}

func (s *Service) validateSessionLocked(state stateFile, sessionID string) error {
	if sessionID == "" {
		return authError(CodeAuthRequired, "缺少管理员 session")
	}
	record, ok := state.Sessions[hashSessionID(sessionID)]
	if !ok || !record.ExpiresAt.After(s.now()) {
		return authError(CodeSessionExpired, "session 已过期")
	}
	return nil
}

func (s *Service) recordFailure(state *stateFile, key string) error {
	if state.Failures == nil {
		state.Failures = map[string]failureRecord{}
	}
	record := state.Failures[key]
	record.Count++
	remaining := maxFailures - record.Count
	if remaining <= 0 {
		record.LockedUntil = s.now().Add(lockDuration)
		record.Count = maxFailures
		state.Failures[key] = record
		if err := s.writeState(*state); err != nil {
			return err
		}
		return &Error{Code: CodeAuthLocked, Message: "登录失败次数过多，请稍后再试", Until: record.LockedUntil}
	}
	state.Failures[key] = record
	if err := s.writeState(*state); err != nil {
		return err
	}
	return &Error{Code: CodeInvalidCredentials, Message: "用户名或密码错误", Remaining: remaining}
}

func (s *Service) loadState() (stateFile, error) {
	if s.statePath == "" {
		return stateFile{}, nil
	}
	data, err := os.ReadFile(filepath.Clean(s.statePath))
	if errors.Is(err, os.ErrNotExist) {
		return stateFile{}, nil
	}
	if err != nil {
		return stateFile{}, err
	}
	if len(data) == 0 {
		return stateFile{}, nil
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return stateFile{}, err
	}
	if state.Sessions == nil {
		state.Sessions = map[string]sessionRecord{}
	}
	if state.Failures == nil {
		state.Failures = map[string]failureRecord{}
	}
	return state, nil
}

func (s *Service) writeState(state stateFile) error {
	if s.statePath == "" {
		return authError(CodeAuthStateNotWritable, "auth state 路径为空")
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := writeStateAtomically(s.statePath, append(data, '\n')); err != nil {
		return authError(CodeAuthStateNotWritable, "auth state 不可写")
	}
	return nil
}

func (s *Service) lock() {
	s.mu.Lock()
}

func (s *Service) unlock() {
	s.mu.Unlock()
}

func (s stateFile) hasAdmin() bool {
	return s.Username != "" && s.PasswordHash != ""
}

func (s stateFile) failure(key string) failureRecord {
	if s.Failures == nil {
		return failureRecord{}
	}
	return s.Failures[key]
}

func (s stateFile) latestLockedUntil(now time.Time) time.Time {
	var latest time.Time
	for _, failure := range s.Failures {
		if failure.LockedUntil.After(now) && failure.LockedUntil.After(latest) {
			latest = failure.LockedUntil
		}
	}
	return latest
}

func failureKey(remoteAddr, username string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return host + "|" + username
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordKeyBytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s$%d$%s$%s",
		passwordAlgorithm,
		passwordIterations,
		base64.RawURLEncoding.EncodeToString(salt),
		base64.RawURLEncoding.EncodeToString(key),
	), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordAlgorithm {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func needsPasswordRehash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordAlgorithm {
		return true
	}
	iter, err := strconv.Atoi(parts[1])
	return err != nil || iter < passwordIterations
}

func randomToken(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashSessionID(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:])
}

func writeStateAtomically(path string, data []byte) error {
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".auth-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, clean); err != nil {
		return err
	}
	_ = os.Chmod(clean, 0o600)
	syncDirBestEffort(dir)
	return nil
}

func syncDirBestEffort(dir string) {
	d, err := os.Open(dir) // #nosec G304 -- dir is the cleaned parent of the configured auth state path.
	if err != nil {
		return
	}
	defer func() { _ = d.Close() }()
	_ = d.Sync()
}
