package typedcfg

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestErrorType_String tests the String method of ErrorType
func TestErrorType_String(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		expected  string
	}{
		{"FileNotFound", ErrorTypeFileNotFound, "FileNotFound"},
		{"ParseFailure", ErrorTypeParseFailure, "ParseFailure"},
		{"ValidationFailure", ErrorTypeValidationFailure, "ValidationFailure"},
		{"WatchFailure", ErrorTypeWatchFailure, "WatchFailure"},
		{"MergeFailure", ErrorTypeMergeFailure, "MergeFailure"},
		{"Unknown", ErrorTypeUnknown, "Unknown"},
		{"InvalidType", ErrorType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.errorType.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConfigError_Error tests the Error method of ConfigError
func TestConfigError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ConfigError
		expected string
	}{
		{
			name: "complete error",
			err: &ConfigError{
				Type:    ErrorTypeParseFailure,
				Source:  "config.yaml",
				Message: "invalid syntax",
				Cause:   fmt.Errorf("yaml: line 5: mapping values are not allowed in this context"),
			},
			expected: "[ParseFailure] source=config.yaml invalid syntax: yaml: line 5: mapping values are not allowed in this context",
		},
		{
			name: "error without cause",
			err: &ConfigError{
				Type:    ErrorTypeFileNotFound,
				Source:  "missing.yaml",
				Message: "file not found",
				Cause:   nil,
			},
			expected: "[FileNotFound] source=missing.yaml file not found",
		},
		{
			name: "minimal error",
			err: &ConfigError{
				Type:    ErrorTypeUnknown,
				Message: "something went wrong",
			},
			expected: "something went wrong",
		},
		{
			name: "error with source only",
			err: &ConfigError{
				Type:   ErrorTypeValidationFailure,
				Source: "config.json",
			},
			expected: "[ValidationFailure] source=config.json",
		},
		{
			name:     "empty error",
			err:      &ConfigError{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConfigError_Unwrap tests the Unwrap method of ConfigError
func TestConfigError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := &ConfigError{
		Type:    ErrorTypeParseFailure,
		Message: "parse failed",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, cause, unwrapped)

	// Test with nil cause
	errNoCause := &ConfigError{
		Type:    ErrorTypeValidationFailure,
		Message: "validation failed",
		Cause:   nil,
	}

	unwrappedNil := errNoCause.Unwrap()
	assert.Nil(t, unwrappedNil)
}

// TestConfigError_Is tests the Is method of ConfigError
func TestConfigError_Is(t *testing.T) {
	err1 := &ConfigError{Type: ErrorTypeParseFailure}
	err2 := &ConfigError{Type: ErrorTypeParseFailure}
	err3 := &ConfigError{Type: ErrorTypeValidationFailure}
	regularErr := fmt.Errorf("regular error")

	// Same error type should match
	assert.True(t, err1.Is(err2))
	// Different error type should not match
	assert.False(t, err1.Is(err3))
	// Non-ConfigError should not match
	assert.False(t, err1.Is(regularErr))
}

// TestNewConfigError tests the NewConfigError function
func TestNewConfigError(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := NewConfigError(ErrorTypeValidationFailure, "config.yaml", "test message", nil)

	assert.Equal(t, ErrorTypeValidationFailure, err.Type)
	assert.Equal(t, "config.yaml", err.Source)
	assert.Equal(t, "test message", err.Message)
	assert.Nil(t, err.Cause)

	// Test with cause
	errWithCause := NewConfigError(ErrorTypeParseFailure, "test.json", "parse error", cause)

	assert.Equal(t, ErrorTypeParseFailure, errWithCause.Type)
	assert.Equal(t, "test.json", errWithCause.Source)
	assert.Equal(t, "parse error", errWithCause.Message)
	assert.Equal(t, cause, errWithCause.Cause)
}

// TestNewParseError tests the NewParseError convenience function
func TestNewParseError(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	parseErr := NewParseError("config.yaml", "parse failed", originalErr)

	assert.Equal(t, ErrorTypeParseFailure, parseErr.Type)
	assert.Equal(t, "config.yaml", parseErr.Source)
	assert.Equal(t, "parse failed", parseErr.Message)
	assert.Equal(t, originalErr, parseErr.Cause)

	// Test unwrapping
	unwrapped := parseErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

// TestNewValidationError tests the NewValidationError convenience function
func TestNewValidationError(t *testing.T) {
	originalErr := fmt.Errorf("validation failed")
	validationErr := NewValidationError("config.json", "invalid field", originalErr)

	assert.Equal(t, ErrorTypeValidationFailure, validationErr.Type)
	assert.Equal(t, "config.json", validationErr.Source)
	assert.Equal(t, "invalid field", validationErr.Message)
	assert.Equal(t, originalErr, validationErr.Cause)
}

func TestErrorTypeNames(t *testing.T) {
	types := []ErrorType{
		ErrorTypeUnknown,
		ErrorTypeFileNotFound,
		ErrorTypeParseFailure,
		ErrorTypeValidationFailure,
		ErrorTypeWatchFailure,
		ErrorTypeMergeFailure,
	}

	expected := []string{
		"Unknown",
		"FileNotFound",
		"ParseFailure",
		"ValidationFailure",
		"WatchFailure",
		"MergeFailure",
	}

	for i, errType := range types {
		assert.Equal(t, expected[i], errType.String())
	}

	// Test unknown error type (beyond defined range)
	unknownType := ErrorType(999)
	assert.Equal(t, "Unknown", unknownType.String())
}
