package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// T-AUTH-001：setup 保存 PBKDF2 密码哈希和 SHA-256 session 哈希（不存明文）
func TestSetupStoresPasswordAndSessionHashes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	svc, err := New(Options{
		StatePath:  path,
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	session, err := svc.Setup(SetupInput{
		Username:   "admin",
		Password:   "long-enough-password",
		SetupToken: "setup-secret",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path 由测试在 t.TempDir 下创建。
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "long-enough-password") || strings.Contains(string(data), session.ID) {
		t.Fatalf("auth state 泄露了明文密钥：%s", data)
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(state.PasswordHash, "$")
	if len(parts) != 4 || parts[0] != passwordAlgorithm || parts[1] != "600000" {
		t.Fatalf("密码哈希格式 = %q", state.PasswordHash)
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("解码 salt：%v", err)
	}
	if len(salt) != passwordSaltBytes {
		t.Fatalf("salt 长度 = %d", len(salt))
	}
	key, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		t.Fatalf("解码 key：%v", err)
	}
	if len(key) != passwordKeyBytes {
		t.Fatalf("key 长度 = %d", len(key))
	}
	if !verifyPassword("long-enough-password", state.PasswordHash) {
		t.Fatal("保存的密码哈希应能通过校验")
	}
	if len(state.Sessions) != 1 {
		t.Fatalf("sessions = %+v", state.Sessions)
	}
}

// T-AUTH-002：同一 IP+用户名登录失败 5 次后锁定
func TestLoginFailureLocksAfterFiveAttempts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	svc, err := New(Options{
		StatePath:  path,
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := svc.Setup(SetupInput{Username: "admin", Password: "long-enough-password", SetupToken: "setup-secret"}); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for i := 0; i < maxFailures-1; i++ {
		err := loginWrongUser(svc)
		authErr, ok := err.(*Error)
		if !ok || authErr.Code != CodeInvalidCredentials {
			t.Fatalf("第 %d 次尝试 error = %T %[2]v", i+1, err)
		}
	}
	err = loginWrongUser(svc)
	authErr, ok := err.(*Error)
	if !ok || authErr.Code != CodeAuthLocked || authErr.Until.IsZero() {
		t.Fatalf("锁定错误 = %T %[1]v", err)
	}

	status, err := svc.Status("")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.LockedUntil.IsZero() {
		t.Fatal("Status().LockedUntil 应报告生效中的登录锁定")
	}
}

// T-AUTH-003：文件只读时，记录失败返回 auth_state_not_writable
func TestRecordFailureReturnsAuthStateNotWritable(t *testing.T) {
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		statePath: filepath.Join(blockingFile, "auth.json"),
		now:       time.Now,
	}
	state := stateFile{}

	err := svc.recordFailure(&state, "127.0.0.1|admin")
	var authErr *Error
	if !errors.As(err, &authErr) || authErr.Code != CodeAuthStateNotWritable {
		t.Fatalf("recordFailure error = %T %[1]v，期望 %s", err, CodeAuthStateNotWritable)
	}
}

func loginWrongUser(svc *Service) error {
	_, err := svc.Login("127.0.0.1:12345", LoginInput{Username: "wrong", Password: "bad-password"})
	return err
}
