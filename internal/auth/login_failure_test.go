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
		t.Fatalf("登录 %s/%s 返回 error=%T %v，期望 auth 错误", remoteAddr, username, err, err)
	}
	if failure.Code != code || failure.Remaining != remaining {
		t.Fatalf("登录 %s/%s 得到 code=%s remaining=%d until=%s，期望 code=%s remaining=%d", remoteAddr, username, failure.Code, failure.Remaining, failure.Until, code, remaining)
	}
	return *failure
}

// T-AUTH-004：失败记录在空闲 15 分钟后过期，服务重启后同样适用。
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

// T-AUTH-005：空闲期从最近一次失败开始计算，计数器按 IP 和用户名相互隔离。
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

// T-AUTH-006：重试不会延长生效中的锁定；锁定到期后恢复完整的失败次数额度。
func TestLogin_LockExpires(t *testing.T) {
	svc, now := loginFailureFixture(t)
	for i := 1; i < 5; i++ {
		requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 5-i)
	}
	locked := requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeAuthLocked, 0)
	until := now.Add(15 * time.Minute)
	if !locked.Until.Equal(until) {
		t.Fatalf("锁定截止 until=%s，期望 %s", locked.Until, until)
	}
	*now = until.Add(-time.Nanosecond)
	_, err := svc.Login("127.0.0.1:1000", LoginInput{Username: "admin", Password: "long-enough-password"})
	var active *Error
	if !errors.As(err, &active) || active.Code != CodeAuthLocked || !active.Until.Equal(until) {
		t.Fatalf("生效中的锁定被改变，或凭据被接受：%v", err)
	}
	*now = until
	status, err := svc.Status("")
	if err != nil || !status.LockedUntil.IsZero() {
		t.Fatalf("锁定过期后 status=%+v err=%v", status, err)
	}
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
}

// T-AUTH-007：认证成功只清除自身对应的失败计数器。
func TestLogin_SuccessClearsFailures(t *testing.T) {
	svc, _ := loginFailureFixture(t)
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 3)
	requireLoginFailure(t, svc, "127.0.0.2:1000", "admin", CodeInvalidCredentials, 4)
	session, err := svc.Login("127.0.0.1:2000", LoginInput{Username: "admin", Password: "long-enough-password"})
	if err != nil || session == nil {
		t.Fatalf("登录失败：session=%v err=%v", session, err)
	}
	if err := svc.ValidateSession(session.ID); err != nil {
		t.Fatalf("新 session 无效：%v", err)
	}
	requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
	requireLoginFailure(t, svc, "127.0.0.2:1000", "admin", CodeInvalidCredentials, 3)
}

// T-AUTH-008：没有失败时间戳的记录既不能保留过时的计数，也不能取消仍在生效的锁定。
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
					t.Fatalf("已有锁定被改变：got=%s want=%s", active.Until, until)
				}
				*now = until
			}
			requireLoginFailure(t, svc, "127.0.0.1:1000", "admin", CodeInvalidCredentials, 4)
		})
	}
}
