package app

import (
	"errors"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

type ValidateResult struct {
	Valid    bool             `json:"valid"`
	Errors   []DiagnosticItem `json:"errors"`
	Warnings []DiagnosticItem `json:"warnings"`
	Infos    []DiagnosticItem `json:"infos"`
}

type DiagnosticItem struct {
	Severity    string            `json:"severity"`
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	DisplayPath string            `json:"display_path"`
	Locator     DiagnosticLocator `json:"locator"`
}

type DiagnosticLocator struct {
	Section     string `json:"section"`
	Key         string `json:"key,omitempty"`
	Index       *int   `json:"index,omitempty"`
	ValuePath   string `json:"value_path,omitempty"`
	JSONPointer string `json:"json_pointer"`
}

func ValidateResultFromError(err error) (*ValidateResult, bool) {
	cfgErrs := collectConfigErrors(err)
	if len(cfgErrs) == 0 {
		return nil, false
	}

	result := &ValidateResult{
		Valid:    false,
		Errors:   make([]DiagnosticItem, 0, len(cfgErrs)),
		Warnings: []DiagnosticItem{},
		Infos:    []DiagnosticItem{},
	}
	for _, cfgErr := range cfgErrs {
		result.Errors = append(result.Errors, diagnosticFromConfigError(cfgErr))
	}
	return result, true
}

func diagnosticFromConfigError(err *errtype.ConfigError) DiagnosticItem {
	section := err.Section
	locator := DiagnosticLocator{
		Section:     section,
		Key:         err.Key,
		Index:       err.Index,
		ValuePath:   err.ValuePath,
		JSONPointer: jsonPointer(section, err.Index, err.ValuePath),
	}
	if section == "" {
		locator.JSONPointer = "/config"
	}
	return DiagnosticItem{
		Severity:    "error",
		Code:        string(err.Code),
		Message:     err.Message,
		DisplayPath: err.DisplayPath(),
		Locator:     locator,
	}
}

func collectConfigErrors(err error) []*errtype.ConfigError {
	leaves := flattenErrors(err)
	result := make([]*errtype.ConfigError, 0, len(leaves))
	for _, leaf := range leaves {
		var cfgErr *errtype.ConfigError
		if errors.As(leaf, &cfgErr) {
			result = append(result, cfgErr)
		}
	}
	return result
}

func flattenErrors(err error) []error {
	if err == nil {
		return nil
	}

	type multiUnwrapper interface{ Unwrap() []error }
	if joined, ok := err.(multiUnwrapper); ok {
		var result []error
		for _, inner := range joined.Unwrap() {
			result = append(result, flattenErrors(inner)...)
		}
		return result
	}
	return []error{err}
}

func jsonPointer(section string, index *int, valuePath string) string {
	if section == "" {
		return "/config"
	}

	segments := []string{"config", section}
	if isOrderedSection(section) && index != nil {
		segments = append(segments, strconv.Itoa(*index))
		if valuePath == "key" {
			segments = append(segments, "key")
			return "/" + strings.Join(escapePointerSegments(segments), "/")
		}
		segments = append(segments, "value")
	} else if index != nil {
		segments = append(segments, strconv.Itoa(*index))
	}
	segments = append(segments, valuePathSegments(valuePath)...)
	return "/" + strings.Join(escapePointerSegments(segments), "/")
}

func isOrderedSection(section string) bool {
	return section == "groups" || section == "routing" || section == "rulesets"
}

func valuePathSegments(path string) []string {
	if path == "" {
		return nil
	}

	var segments []string
	for _, part := range strings.Split(path, ".") {
		for part != "" {
			bracket := strings.IndexByte(part, '[')
			switch {
			case bracket < 0:
				segments = append(segments, part)
				part = ""
			case bracket > 0:
				segments = append(segments, part[:bracket])
				part = part[bracket:]
			default:
				end := strings.IndexByte(part, ']')
				if end < 0 {
					segments = append(segments, strings.Trim(part, "[]"))
					part = ""
					continue
				}
				segments = append(segments, part[1:end])
				part = part[end+1:]
			}
		}
	}
	return segments
}

func escapePointerSegments(segments []string) []string {
	escaped := make([]string, len(segments))
	for i, segment := range segments {
		segment = strings.ReplaceAll(segment, "~", "~0")
		segment = strings.ReplaceAll(segment, "/", "~1")
		escaped[i] = segment
	}
	return escaped
}
