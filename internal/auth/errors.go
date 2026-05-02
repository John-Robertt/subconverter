package auth

import "time"

type Error struct {
	Code      string
	Message   string
	Remaining int
	Until     time.Time
}

func (e *Error) Error() string {
	return e.Message
}

func authError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

const (
	CodeAuthRequired         = "auth_required"
	CodeSessionExpired       = "session_expired"
	CodeInvalidCredentials   = "invalid_credentials"
	CodeAuthLocked           = "auth_locked"
	CodeSetupTokenRequired   = "setup_token_required"
	CodeSetupTokenInvalid    = "setup_token_invalid"
	CodeSetupNotAllowed      = "setup_not_allowed"
	CodeAuthStateNotWritable = "auth_state_not_writable"
	CodePasswordTooWeak      = "password_too_weak"
)
