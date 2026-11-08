package app

import "github.com/John-Robertt/subconverter/internal/errtype"

func configNotInitializedError() error {
	return &errtype.ConfigError{Code: errtype.CodeConfigNotInitialized, Message: "配置尚未生效，请先导入或编辑配置，保存后热重载"}
}

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
