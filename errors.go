// Package typedcfg provides configuration management with comprehensive error handling.
// This file defines error types and structures for detailed error reporting
// across different configuration operations.
package typedcfg

import (
	"fmt"
	"strings"
)

// ErrorType represents the category of configuration errors.
// It provides a way to classify different types of failures that can occur
// during configuration loading, parsing, validation, and management.
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

// String returns the string representation of the error type.
// This method implements the Stringer interface for better error reporting
// and debugging purposes.
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

// ConfigError represents a structured configuration error with detailed context.
// It provides information about the error type, source, descriptive message,
// and the underlying cause for comprehensive error reporting.
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

// Error implements the error interface by returning a formatted error message.
// The message includes the error type, source, and descriptive text for
// comprehensive error reporting.
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

// Unwrap 返回底层错误
func (e *ConfigError) Unwrap() error {
	return e.Cause
}

// Is 检查错误类型
func (e *ConfigError) Is(target error) bool {
	if ce, ok := target.(*ConfigError); ok {
		return e.Type == ce.Type
	}
	return false
}

// NewConfigError 创建新的配置错误
func NewConfigError(errType ErrorType, source, message string, cause error) *ConfigError {
	return &ConfigError{
		Type:    errType,
		Source:  source,
		Message: message,
		Cause:   cause,
	}
}

// Convenience functions for creating errors

// NewParseError 创建解析错误
func NewParseError(source, message string, cause error) *ConfigError {
	return NewConfigError(ErrorTypeParseFailure, source, message, cause)
}

// NewValidationError 创建验证错误
func NewValidationError(source, message string, cause error) *ConfigError {
	return NewConfigError(ErrorTypeValidationFailure, source, message, cause)
}
