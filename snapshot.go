package typedcfg

import "github.com/nextpkg/typedcfg/internal/snapshot"

func snapshotCopy[T any](value *T) (*T, error) { return snapshot.Copy(value) }
