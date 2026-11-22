// Package defaults applies default tags to zero-valued struct fields.
package defaults

import "github.com/nextpkg/typedcfg/internal/defaulttags"

// SetDefaults fills zero-valued fields from their default tags and recurses into
// nested value structs. Nil pointers require their own default tag to be allocated;
// existing non-nil pointers and collection elements are not traversed.
//
// Supported tag format: `default:"value"`
//
// Examples:
//
//	type Config struct {
//	    Port     int           `default:"8080"`
//	    Host     string        `default:"localhost"`
//	    Timeout  time.Duration `default:"30s"`
//	    Debug    bool          `default:"false"`
//	    Tags     []string      `default:"tag1,tag2,tag3"`
//	}
//
// Inputs other than a pointer to a struct are ignored. Tag parsing errors are
// returned; numeric defaults must fit their destination types.
func SetDefaults(ptr any) error {
	return defaulttags.Apply(ptr, nil)
}
