package advanced

import "github.com/nextpkg/typedcfg"

type ErrorType = typedcfg.ErrorType
type ConfigError = typedcfg.ConfigError

const (
	ErrorTypeUnknown           = typedcfg.ErrorTypeUnknown
	ErrorTypeFileNotFound      = typedcfg.ErrorTypeFileNotFound
	ErrorTypeParseFailure      = typedcfg.ErrorTypeParseFailure
	ErrorTypeValidationFailure = typedcfg.ErrorTypeValidationFailure
	ErrorTypeWatchFailure      = typedcfg.ErrorTypeWatchFailure
	ErrorTypeMergeFailure      = typedcfg.ErrorTypeMergeFailure
)

var NewConfigError = typedcfg.NewConfigError
var NewParseError = typedcfg.NewParseError
var NewValidationError = typedcfg.NewValidationError
