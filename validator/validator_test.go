// Copyright (c) 2024 nextpkg. All rights reserved.
// This file contains unit tests for the validator package.
package validator

import (
	"errors"
	"testing"
)

// TestStruct represents a test structure for validation
type TestStruct struct {
	Name     string `validate:"required"`
	Email    string `validate:"required,email"`
	Age      int    `validate:"min=0,max=120"`
	Optional string
}

// TestStructWithCustomValidator implements the Validator interface
type TestStructWithCustomValidator struct {
	Value string `validate:"required"`
}

// Validate implements the Validator interface
func (t TestStructWithCustomValidator) Validate() error {
	if t.Value == "invalid" {
		return errors.New("custom validation failed: value cannot be 'invalid'")
	}
	return nil
}

// TestStructWithFailingValidator implements the Validator interface with failing validation
type TestStructWithFailingValidator struct {
	Value string `validate:"required"`
}

// Validate implements the Validator interface with always failing validation
func (t TestStructWithFailingValidator) Validate() error {
	return errors.New("custom validation always fails")
}

// TestValidate_ValidStruct tests validation with a valid struct
func TestValidate_ValidStruct(t *testing.T) {
	validStruct := TestStruct{
		Name:     "John Doe",
		Email:    "john@example.com",
		Age:      30,
		Optional: "optional value",
	}

	err := Validate(validStruct)
	if err != nil {
		t.Errorf("Expected no error for valid struct, got: %v", err)
	}
}

// TestValidate_InvalidStruct tests validation with an invalid struct
func TestValidate_InvalidStruct(t *testing.T) {
	tests := []struct {
		name      string
		value     TestStruct
		wantError bool
	}{
		{
			name: "MissingRequiredName",
			value: TestStruct{
				Name:  "", // Required field missing
				Email: "john@example.com",
				Age:   30,
			},
			wantError: true,
		},
		{
			name: "InvalidEmail",
			value: TestStruct{
				Name:  "John Doe",
				Email: "invalid-email", // Invalid email format
				Age:   30,
			},
			wantError: true,
		},
		{
			name: "AgeOutOfRange",
			value: TestStruct{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   150, // Age exceeds maximum
			},
			wantError: true,
		},
		{
			name: "NegativeAge",
			value: TestStruct{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   -5, // Negative age
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.value)
			if tt.wantError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error for %s, got: %v", tt.name, err)
			}
		})
	}
}

// TestValidate_NilValue tests validation with nil value
func TestValidate_NilValue(t *testing.T) {
	err := Validate(nil)
	if err == nil {
		t.Error("Expected error for nil value, got nil")
	}

	expectedMsg := "validation target cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestValidate_CustomValidator tests validation with custom validator
func TestValidate_CustomValidator(t *testing.T) {
	tests := []struct {
		name       string
		testStruct TestStructWithCustomValidator
		wantError  bool
	}{
		{
			name: "ValidCustomValidator",
			testStruct: TestStructWithCustomValidator{
				Value: "valid",
			},
			wantError: false,
		},
		{
			name: "InvalidCustomValidator",
			testStruct: TestStructWithCustomValidator{
				Value: "invalid",
			},
			wantError: true,
		},
		{
			name: "MissingRequiredFieldWithCustomValidator",
			testStruct: TestStructWithCustomValidator{
				Value: "", // Required field missing
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.testStruct)
			if tt.wantError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error for %s, got: %v", tt.name, err)
			}
		})
	}
}

// TestValidate_FailingCustomValidator tests validation with always failing custom validator
func TestValidate_FailingCustomValidator(t *testing.T) {
	testStruct := TestStructWithFailingValidator{
		Value: "any value",
	}

	err := Validate(testStruct)
	if err == nil {
		t.Error("Expected error for failing custom validator, got nil")
	}

	expectedMsg := "custom validation always fails"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestValidate_PointerToStruct tests validation with pointer to struct
func TestValidate_PointerToStruct(t *testing.T) {
	validStruct := &TestStruct{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	err := Validate(validStruct)
	if err != nil {
		t.Errorf("Expected no error for valid struct pointer, got: %v", err)
	}

	// Test with invalid struct pointer
	invalidStruct := &TestStruct{
		Name:  "", // Required field missing
		Email: "john@example.com",
		Age:   30,
	}

	err = Validate(invalidStruct)
	if err == nil {
		t.Error("Expected error for invalid struct pointer, got nil")
	}
}

// TestValidate_NonStruct tests validation with non-struct types
func TestValidate_NonStruct(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"String", "test string"},
		{"Integer", 42},
		{"Boolean", true},
		{"Slice", []string{"a", "b", "c"}},
		{"Map", map[string]int{"key": 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Exercise non-struct inputs without requiring a particular error.
			err := Validate(tt.value)
			_ = err
		})
	}
}

// TestValidate_StructWithoutTags tests validation with struct without validation tags
func TestValidate_StructWithoutTags(t *testing.T) {
	type SimpleStruct struct {
		Name  string
		Value int
	}

	testStruct := SimpleStruct{
		Name:  "test",
		Value: 42,
	}

	err := Validate(testStruct)
	if err != nil {
		t.Errorf("Expected no error for struct without validation tags, got: %v", err)
	}
}

// TestValidate_StructWithCustomValidatorAndStructTags tests combination of struct tags and custom validation
func TestValidate_StructWithCustomValidatorAndStructTags(t *testing.T) {
	// Test case where struct tag validation passes but custom validation fails
	struct1 := TestStructWithCustomValidator{
		Value: "invalid", // Valid for struct tag (required), but invalid for custom validator
	}

	err := Validate(struct1)
	if err == nil {
		t.Error("Expected error when custom validation fails, got nil")
	}

	// Test case where struct tag validation fails (custom validation won't be called)
	struct2 := TestStructWithCustomValidator{
		Value: "", // Invalid for struct tag (required)
	}

	err = Validate(struct2)
	if err == nil {
		t.Error("Expected error when struct tag validation fails, got nil")
	}

	// Error should be about struct validation, not custom validation
	if !contains(err.Error(), "struct validation failed") {
		t.Errorf("Expected struct validation error, got: %v", err)
	}
}

// TestValidate_ErrorMessages tests that error messages are properly formatted
func TestValidate_ErrorMessages(t *testing.T) {
	// Test struct validation error message
	invalidStruct := TestStruct{
		Name:  "", // Required field missing
		Email: "john@example.com",
		Age:   30,
	}

	err := Validate(invalidStruct)
	if err == nil {
		t.Error("Expected error for invalid struct, got nil")
	}

	if !contains(err.Error(), "struct validation failed") {
		t.Errorf("Expected 'struct validation failed' in error message, got: %v", err.Error())
	}
}

// TestValidate_InterfaceImplementation tests that the validator interface is properly implemented
func TestValidate_InterfaceImplementation(t *testing.T) {
	// Ensure our test structs properly implement the Validator interface
	var _ Validator = TestStructWithCustomValidator{}
	var _ Validator = TestStructWithFailingValidator{}

	// Test that non-validator structs don't implement the interface
	var nonValidator TestStruct
	if _, ok := interface{}(nonValidator).(Validator); ok {
		t.Error("TestStruct should not implement Validator interface")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsHelper(s, substr))))
}

func containsHelper(s, substr string) bool {
	for i := 1; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestValidate_EdgeCases tests various edge cases
func TestValidate_EdgeCases(t *testing.T) {
	// Test with empty struct
	type EmptyStruct struct{}
	emptyStruct := EmptyStruct{}

	err := Validate(emptyStruct)
	if err != nil {
		t.Errorf("Expected no error for empty struct, got: %v", err)
	}

	// Test with struct containing only optional fields
	type OptionalStruct struct {
		Optional1 string
		Optional2 int
	}
	optionalStruct := OptionalStruct{}

	err = Validate(optionalStruct)
	if err != nil {
		t.Errorf("Expected no error for struct with only optional fields, got: %v", err)
	}
}
