package typedcfg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
		if err := createFileExclusive(src.path, data); err != nil {
			return fmt.Errorf("create configuration %s: %w", src.path, err)
		}
	}
	return nil
}

// createFileExclusive publishes a fully written file without replacing a file
// created by another process. The temporary file lives on the same filesystem.
func createFileExclusive(path string, data []byte) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".typedcfg-*")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.Remove(f.Name())) }()
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Link(f.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}
