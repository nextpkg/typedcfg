package advanced

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/nextpkg/typedcfg/internal/fileutil"

	"github.com/go-viper/mapstructure/v2"
	"github.com/nextpkg/typedcfg/defaults"
)

func (b *Builder[T]) createMissingFiles(ctx context.Context) error {
	for _, src := range b.sources {
		if src.path == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// Lstat treats existing symlinks (including dangling ones) as existing files.
		if _, err := os.Lstat(src.path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect configuration %s: %w", src.path, err)
		}
		var cfg T
		if err := defaults.SetDefaults(&cfg); err != nil {
			return NewParseError("defaults", "cannot generate configuration", err)
		}
		var values map[string]any
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: b.tag, Result: &values})
		if err != nil {
			return err
		}
		if err := decoder.Decode(cfg); err != nil {
			return fmt.Errorf("encode default configuration: %w", err)
		}
		data, err := src.parser.Marshal(values)
		if err != nil {
			return fmt.Errorf("encode %s: %w", src.path, err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fileutil.CreateExclusive(src.path, data); err != nil {
			return fmt.Errorf("create configuration %s: %w", src.path, err)
		}
	}
	return nil
}
