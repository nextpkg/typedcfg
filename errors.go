package typedcfg

import (
	"fmt"
	"strings"
)

// ErrorType categorizes a configuration failure. Loading currently wraps source
// and decoding failures as ParseFailure and validation failures as ValidationFailure;
// other categories are available to callers constructing their own ConfigError.
type ErrorType int

const (
	// ErrorTypeUnknown represents an unclassified error
	ErrorTypeUnknown ErrorType = iota
	// ErrorTypeFileNotFound indicates a configuration file could not be found
	ErrorTypeFileNotFound
	// ErrorTypeParseFailure indicates failure to parse configuration data
	ErrorTypeParseFailure
	// ErrorTypeValidationFailure indicates configuration validation failed
	ErrorTypeValidationFailure
	// ErrorTypeWatchFailure indicates failure in file watching functionality
	ErrorTypeWatchFailure
	// ErrorTypeMergeFailure indicates failure to merge configuration sources
	ErrorTypeMergeFailure
)

// String returns the category name.
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeFileNotFound:
		return "FileNotFound"
	case ErrorTypeParseFailure:
		return "ParseFailure"
	case ErrorTypeValidationFailure:
		return "ValidationFailure"
	case ErrorTypeWatchFailure:
		return "WatchFailure"
	case ErrorTypeMergeFailure:
		return "MergeFailure"
	default:
		return "Unknown"
	}
}

// ConfigError identifies a configuration failure and wraps its underlying cause.
type ConfigError struct {
	// Type categorizes the kind of error that occurred
	Type ErrorType
	// Source identifies where the error originated (file path, provider name, etc.)
	Source string
	// Message provides a human-readable description of the error
	Message string
	// Cause holds the underlying error that triggered this configuration error
	Cause error
}

// Error includes the category, source, message, and underlying cause.
func (e *ConfigError) Error() string {
	var parts []string

	if e.Type != ErrorTypeUnknown {
		parts = append(parts, fmt.Sprintf("[%s]", e.Type.String()))
	}

	if e.Source != "" {
		parts = append(parts, fmt.Sprintf("source=%s", e.Source))
	}

	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	result := strings.Join(parts, " ")

	if e.Cause != nil {
		result += fmt.Sprintf(": %v", e.Cause)
	}

	return result
}

// Unwrap returns the underlying cause.
func (e *ConfigError) Unwrap() error {
	return e.Cause
}

// Is matches another ConfigError by category.
func (e *ConfigError) Is(target error) bool {
	if ce, ok := target.(*ConfigError); ok {
		return e.Type == ce.Type
	}
	return false
}

// NewConfigError constructs a categorized error.
func NewConfigError(errType ErrorType, source, message string, cause error) *ConfigError {
	return &ConfigError{
		Type:    errType,
		Source:  source,
		Message: message,
		Cause:   cause,
	}
}

// NewParseError wraps a source, decoding, or default-value error.
func NewParseError(source, message string, cause error) *ConfigError {
	return NewConfigError(ErrorTypeParseFailure, source, message, cause)
}

// NewValidationError wraps a validation error.
func NewValidationError(source, message string, cause error) *ConfigError {
	return NewConfigError(ErrorTypeValidationFailure, source, message, cause)
}
