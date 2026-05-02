package app

type BadRequestError struct {
	Code    string
	Message string
}

func newBadRequestError(code, message string) *BadRequestError {
	return &BadRequestError{Code: code, Message: message}
}

func (e *BadRequestError) Error() string {
	return e.Message
}
