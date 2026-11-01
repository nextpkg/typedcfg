package typedcfg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type isolatedConfig struct {
	Values map[string][]int `json:"values"`
	Names  []string         `json:"names"`
	Child  *struct {
		Count int `json:"count"`
	} `json:"child"`
	Dynamic any `json:"dynamic"`
}

func TestSnapshotIsolatesNestedSnapshots(t *testing.T) {
	cm, err := LoadSource[isolatedConfig](t.Context(), func(context.Context) ([]byte, error) {
		return []byte(`{"values":{"a":[1,2]},"names":["first"],"child":{"count":7},"dynamic":{"list":["original"]}}`), nil
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, cm.Close()) }()
	first, second := cm.Snapshot(), cm.Snapshot()
	require.NotSame(t, first, second)
	first.Values["a"][0] = 99
	first.Values["extra"] = []int{5}
	first.Names[0] = "changed"
	first.Child.Count = 9
	first.Dynamic.(map[string]any)["list"].([]any)[0] = "changed"
	require.Equal(t, second, cm.Get())
	require.NoError(t, cm.Reload())
	require.Equal(t, second, cm.Get())
	require.Equal(t, 99, first.Values["a"][0])

	// Independent readers may mutate their own copy while Reload and Close run.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 30 {
				local := cm.Snapshot()
				local.Values["a"][0]++
				local.Child.Count++
				local.Names[0] = "private"
				local.Dynamic.(map[string]any)["list"].([]any)[0] = "private"
				if err := cm.Reload(); err != nil && !errors.Is(err, ErrClosed) && !errors.Is(err, context.Canceled) {
					t.Error(err)
				}
			}
			if err := cm.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	require.Equal(t, second, cm.Get())
}

type snapshotNode struct {
	Next  *snapshotNode
	Data  map[string]any
	Slice []any
	At    time.Time
}

func TestSnapshotCyclesAndTime(t *testing.T) {
	at := time.Now()
	node := &snapshotNode{At: at, Data: map[string]any{}, Slice: make([]any, 1)}
	node.Next = node
	node.Data["self"] = node.Data
	node.Slice[0] = node.Slice
	copy, err := snapshotCopy(node)
	require.NoError(t, err)
	require.NotSame(t, node, copy)
	require.Same(t, copy, copy.Next)
	require.Equal(t, at, copy.At)
	copy.Data["self"].(map[string]any)["new"] = true
	require.Equal(t, true, copy.Data["new"])
	require.NotContains(t, node.Data, "new")
	copy.Slice[0].([]any)[0] = "changed"
	require.Equal(t, "changed", copy.Slice[0])
	require.IsType(t, []any{}, node.Slice[0])
}

type privateConfig struct{ hidden []int }

func (c *privateConfig) Validate() error { c.hidden = []int{1}; return nil }

type dynamicConfig struct{ Value any }

var dynamicUnsupported bool

func (c *dynamicConfig) Validate() error {
	if dynamicUnsupported {
		c.Value = make(chan int)
	} else {
		c.Value = []int{1}
	}
	return nil
}

func TestUncopyableConfigurationRejected(t *testing.T) {
	_, err := Load[privateConfig]("")
	require.ErrorContains(t, err, "unexported field")
	_, err = Load[struct{ Signal chan int }]("")
	require.ErrorContains(t, err, "cannot isolate")
	_, err = Load[struct{ Lock sync.Mutex }]("")
	require.ErrorContains(t, err, "synchronization")
	cm, err := Load[dynamicConfig]("")
	require.NoError(t, err)
	defer func() { require.NoError(t, cm.Close()) }()
	old := cm.Get()
	dynamicUnsupported = true
	defer func() { dynamicUnsupported = false }()
	require.ErrorContains(t, cm.Reload(), "cannot isolate")
	require.Equal(t, old, cm.Get())
}

// Shared snapshots stay consistent while readers overlap publication.
func TestGetSharedSnapshot(t *testing.T) {
	type config struct {
		Version int
		Values  []int
	}
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"Version":0,"Values":[0]}`), 0600))
	cm := MustLoad[config](path)
	defer func() { require.NoError(t, cm.Close()) }()
	first := cm.Get()
	require.Same(t, first, cm.Get())
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = cm.Get() }))
	var wg sync.WaitGroup
	done := make(chan struct{})
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
					cfg := cm.Get()
					if cfg.Version != cfg.Values[0] {
						t.Error("inconsistent snapshot")
						return
					}
				}
			}
		})
	}
	defer func() { close(done); wg.Wait() }()
	for i := 1; i <= 10; i++ {
		require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, `{"Version":%d,"Values":[%d]}`, i, i), 0600))
		require.NoError(t, cm.Reload())
	}
	require.NotSame(t, first, cm.Get())
	require.Equal(t, 0, first.Values[0])
	require.Equal(t, 10, cm.Get().Version)
	require.NoError(t, cm.Close())
	require.Equal(t, 10, cm.Get().Version)
	var uninitialized ConfigManager[config]
	require.Nil(t, uninitialized.Get())
	require.Nil(t, uninitialized.Snapshot())
	var absent *ConfigManager[config]
	require.Nil(t, absent.Get())
	require.Nil(t, absent.Snapshot())
}
