// Package validator runs struct-tag validation followed by custom validation.
package validator

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// vld also applies required tags to non-pointer struct fields.
var vld = validator.New(validator.WithRequiredStructEnabled())

// Validator supplies application-specific validation after tag validation succeeds.
type Validator interface {
	// Validate performs custom validation and returns an error if validation fails
	Validate() error
}

// Validate runs go-playground/validator struct tags, then calls Validate on v
// when it implements Validator. It returns the first error.
func Validate(v any) error {
	if v == nil {
		return fmt.Errorf("validation target cannot be nil")
	}

	err := vld.Struct(v)
	if err != nil {
		return fmt.Errorf("struct validation failed: %w", err)
	}

	if val, ok := v.(Validator); ok {
		return val.Validate()
	}

	return nil
}
