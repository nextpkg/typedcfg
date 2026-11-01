package validator

import "github.com/nextpkg/typedcfg"

// Option enables validate struct tags. The core calls custom Validate separately.
func Option() typedcfg.Option { return typedcfg.WithValidation(vld.Struct) }
