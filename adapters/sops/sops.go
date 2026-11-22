// Package sops provides a typedcfg source for files encrypted with SOPS.
package sops

import (
	"context"
	"fmt"

	"github.com/getsops/sops/v3/decrypt"
	"github.com/nextpkg/typedcfg"
)

// File returns a source that reads and decrypts path on every load.
// The format describes the SOPS input, for example "json", "yaml", or "binary".
// An empty format uses SOPS's path-based format detection. Select the plaintext
// codec separately when calling typedcfg.LoadSource.
//
// Decryption uses SOPS's normal key configuration. In-flight reads and key
// operations are subject to SOPS and its key providers' own behavior because
// the upstream API does not accept a context.
func File(path, format string) typedcfg.Source {
	return func(ctx context.Context) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		data, err := decrypt.File(path, format)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			return nil, fmt.Errorf("sops source %q: %w", path, err)
		}
		return data, nil
	}
}
