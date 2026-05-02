package app

import "github.com/John-Robertt/subconverter/internal/generate"

type GenerateInput = generate.Request
type GenerateResult = generate.Result

func NewGenerateInput(format, rawFilename string, filenamePresent bool) (GenerateInput, error) {
	if !generate.ValidFormat(format) {
		return GenerateInput{}, newBadRequestError("invalid_request", "format 参数无效：必须为 clash 或 surge")
	}
	filename, err := generate.ResolveFilename(rawFilename, filenamePresent, format)
	if err != nil {
		return GenerateInput{}, newBadRequestError("invalid_request", err.Error())
	}
	return GenerateInput{Format: format, Filename: filename}, nil
}
