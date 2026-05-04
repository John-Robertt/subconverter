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

	data, err := os.ReadFile(path) // #nosec G304 -- path is created under t.TempDir by the test.
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "long-enough-password") || strings.Contains(string(data), session.ID) {
		t.Fatalf("auth state leaked plaintext secret: %s", data)
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(state.PasswordHash, "$")
	if len(parts) != 4 || parts[0] != passwordAlgorithm || parts[1] != "600000" {
		t.Fatalf("password hash format = %q", state.PasswordHash)
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode salt: %v", err)
	}
	if len(salt) != passwordSaltBytes {
		t.Fatalf("salt len = %d", len(salt))
	}
	key, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	if len(key) != passwordKeyBytes {
		t.Fatalf("key len = %d", len(key))
	}
	if !verifyPassword("long-enough-password", state.PasswordHash) {
		t.Fatal("stored password hash should verify")
	}
	if len(state.Sessions) != 1 {
		t.Fatalf("sessions = %+v", state.Sessions)
	}
}

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
			t.Fatalf("attempt %d error = %T %[2]v", i+1, err)
		}
	}
	err = loginWrongUser(svc)
	authErr, ok := err.(*Error)
	if !ok || authErr.Code != CodeAuthLocked || authErr.Until.IsZero() {
		t.Fatalf("lock error = %T %[1]v", err)
	}

	status, err := svc.Status("")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.LockedUntil.IsZero() {
		t.Fatal("Status().LockedUntil should report active login lock")
	}
}

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
		t.Fatalf("recordFailure error = %T %[1]v, want %s", err, CodeAuthStateNotWritable)
	}
}

func loginWrongUser(svc *Service) error {
	_, err := svc.Login("127.0.0.1:12345", LoginInput{Username: "wrong", Password: "bad-password"})
	return err
}
