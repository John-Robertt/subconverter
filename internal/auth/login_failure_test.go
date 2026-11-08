package auth

import (
	"errors"
	"io"
	"log"
	"path/filepath"
	"testing"
	"time"
)

func loginFailureFixture(t *testing.T) (*Service, *time.Time) {
	t.Helper()
	now := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	svc, err := New(Options{
		StatePath: filepath.Join(t.TempDir(), "auth.json"), SetupToken: "setup-secret",
		Logger: log.New(io.Discard, "", 0), Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Setup(SetupInput{Username: "admin", Password: "long-enough-password", SetupToken: "setup-secret"}); err != nil {
		t.Fatal(err)
	}
	return svc, &now
}

func requireLoginFailure(t *testing.T, svc *Service, remoteAddr, username, code string, remaining int) Error {
	t.Helper()
	_, err := svc.Login(remoteAddr, LoginInput{Username: username, Password: "wrong-password"})
	var failure *Error
	if !errors.As(err, &failure) {
		t.Fatalf("login %s/%s error=%T %v want auth error", remoteAddr, username, err, err)
	}
	if failure.Code != code || failure.Remaining != remaining {
		t.Fatalf("login %s/%s got code=%s remaining=%d until=%s want code=%s remaining=%d", remoteAddr, username, failure.Code, failure.Remaining, failure.Until, code, remaining)
	}
	return *failure
}

// T-AUTH-004: failed attempts expire after 15 idle minutes, including across service restarts.
func TestLogin_FailureIdleExpiry(t *testing.T) {
	for _, tt := range []struct {
		name      string
		attempts  int
		delay     time.Duration
		remaining int
	}{
		{"before deadline", 2, 15*time.Minute - time.Nanosecond, 2},
		{"at deadline", 2, 15 * time.Minute, 4},
		{"one day later", 2, 24 * time.Hour, 4},
		{"four failures then one day later", 4, 24 * time.Hour, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, now := loginFailureFixture(t)
			for i := 1; i <= tt.attempts; i++ {
				requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 5-i)
			}
			*now = now.Add(tt.delay)
			restarted, err := New(Options{StatePath: svc.statePath, Now: svc.now})
			if err != nil {
				t.Fatal(err)
			}
			requireLoginFailure(t, restarted, "127.0.0.1:3000", "admin", CodeInvalidCredentials, tt.remaining)
		})
	}
}

// T-AUTH-005: the idle period starts at the most recent failure and counters remain isolated by IP and username.
func TestLogin_FailureIdlePeriodAndIsolation(t *testing.T) {
	svc, now := loginFailureFixture(t)
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
	*now = now.Add(14 * time.Minute)
	requireLoginFailure(t, svc, "127.0.0.1:2000", "admin", CodeInvalidCredentials, 3)
	*now = now.Add(14 * time.Minute)
	requireLoginFailure(t, svc, "127.0.0.1:3000", "admin", CodeInvalidCredentials, 2)
	requireLoginFailure(t, svc, "127.0.0.2:1000", "admin", CodeInvalidCredentials, 4)
	requireLoginFailure(t, svc, "127.0.0.1:1000", "other", CodeInvalidCredentials, 4)
	*now = now.Add(15 * time.Minute)
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
}

// T-AUTH-006: active locks cannot be extended by retries; their expiry restores a fresh failure budget.
func TestLogin_LockExpires(t *testing.T) {
	svc, now := loginFailureFixture(t)
	for i := 1; i < 5; i++ {
		requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 5-i)
	}
	locked := requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeAuthLocked, 0)
	until := now.Add(15 * time.Minute)
	if !locked.Until.Equal(until) {
		t.Fatalf("locked until=%s want %s", locked.Until, until)
	}
	*now = until.Add(-time.Nanosecond)
	_, err := svc.Login("127.0.0.1:1000", LoginInput{Username: "admin", Password: "long-enough-password"})
	var active *Error
	if !errors.As(err, &active) || active.Code != CodeAuthLocked || !active.Until.Equal(until) {
		t.Fatalf("active lock changed or accepted credentials: %v", err)
	}
	*now = until
	status, err := svc.Status("")
	if err != nil || !status.LockedUntil.IsZero() {
		t.Fatalf("expired lock status=%+v err=%v", status, err)
	}
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
}

// T-AUTH-007: successful authentication clears only its own active failure counter.
func TestLogin_SuccessClearsFailures(t *testing.T) {
	svc, _ := loginFailureFixture(t)
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 3)
	requireLoginFailure(t, svc, "127.0.0.2:1000", "admin", CodeInvalidCredentials, 4)
	session, err := svc.Login("127.0.0.1:2000", LoginInput{Username: "admin", Password: "long-enough-password"})
	if err != nil || session == nil {
		t.Fatalf("login failed: session=%v err=%v", session, err)
	}
	if err := svc.ValidateSession(session.ID); err != nil {
		t.Fatalf("new session is invalid: %v", err)
	}
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
	requireLoginFailure(t, svc, "127.0.0.2:1000", "admin", CodeInvalidCredentials, 3)
}

// T-AUTH-008: records without failure timestamps cannot preserve stale counts or cancel a still-active lock.
func TestLogin_FailuresWithoutTimestamp(t *testing.T) {
	for _, locked := range []bool{false, true} {
		t.Run(map[bool]string{false: "unlocked", true: "active lock"}[locked], func(t *testing.T) {
			svc, now := loginFailureFixture(t)
			state, err := svc.loadState()
			if err != nil {
				t.Fatal(err)
			}
			failure := failureRecord{Count: 4}
			until := now.Add(10 * time.Minute)
			if locked {
				failure.Count, failure.LockedUntil = 5, until
			}
			state.Failures[failureKey("127.0.0.1:1000", "admin")] = failure
			if err := svc.writeState(state); err != nil {
				t.Fatal(err)
			}
			if locked {
				active := requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeAuthLocked, 0)
				if !active.Until.Equal(until) {
					t.Fatalf("existing lock changed: got=%s want=%s", active.Until, until)
				}
				*now = until
			}
			requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
		})
	}
}
