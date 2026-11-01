package typedcfg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Port     int  `json:"port" default:"8080" validate:"min=1,max=65535"`
	MaxConns int  `json:"max_conns" default:"10"`
	Enabled  bool `json:"enabled" default:"true"`
}

func writeConfig(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(data), 0600))
}

func TestReloadSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeConfig(t, path, `{"port":9000,"max_conns":25,"enabled":false}`)
	cm, err := Load[testConfig](path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	old := cm.Get()
	require.Equal(t, &testConfig{Port: 9000, MaxConns: 25, Enabled: false}, old)
	for _, invalid := range []string{`{`, `{"port":-1}`} {
		writeConfig(t, path, invalid)
		require.Error(t, cm.Reload())
		require.Same(t, old, cm.Get())
	}
	writeConfig(t, path, `{}`)
	require.NoError(t, cm.Reload())
	require.Equal(t, &testConfig{Port: 8080, MaxConns: 10, Enabled: true}, cm.Get())
	require.Equal(t, 9000, old.Port)
	require.NoError(t, cm.Close())
	require.ErrorIs(t, cm.Reload(), ErrClosed)
	require.Equal(t, 8080, cm.Get().Port)
}

func TestBuilderInputs(t *testing.T) {
	cm, err := Load[testConfig]()
	require.NoError(t, err)
	require.Equal(t, 8080, cm.Get().Port)
	require.NoError(t, cm.Close())
	var nilManager *ConfigManager[testConfig]
	require.Nil(t, nilManager.Get())
	require.NoError(t, nilManager.Close())
	require.Nil(t, (&ConfigManager[testConfig]{}).Get())

	_, err = NewBuilder[testConfig]().AddFile("config.toml").Build(t.Context())
	require.ErrorContains(t, err, "unsupported")
	_, err = NewBuilder[testConfig]().AddProvider(nil, nil).Build(t.Context())
	require.Error(t, err)
	_, err = NewBuilder[int]().Build(t.Context())
	require.Error(t, err)
	_, err = NewBuilder[testConfig]().WithWatch().Build(t.Context())
	require.ErrorContains(t, err, "AddFile")
	_, err = NewBuilder[testConfig]().AddFile(filepath.Join(t.TempDir(), "missing", "config.json")).WithWatch().Build(t.Context())
	require.Error(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = NewBuilder[testConfig]().Build(ctx)
	require.ErrorIs(t, err, context.Canceled)

	cm, err = NewBuilder[testConfig]().AddProvider(rawbytes.Provider([]byte(`{"max_conns":42}`)), json.Parser()).Build(t.Context())
	require.NoError(t, err)
	require.Equal(t, 42, cm.Get().MaxConns)
	require.NoError(t, cm.Close())
	type Legacy struct {
		MaxConns int `koanf:"max_conns"`
	}
	legacy, err := NewBuilder[Legacy]().WithTag("koanf").AddProvider(rawbytes.Provider([]byte(`{"max_conns":42}`)), json.Parser()).Build(t.Context())
	require.NoError(t, err)
	require.Equal(t, 42, legacy.Get().MaxConns)
	require.NoError(t, legacy.Close())
}

func TestEnvironmentUnderscores(t *testing.T) {
	type Config struct {
		Database struct {
			MaxConns int `json:"max_conns"`
		} `json:"database"`
	}
	t.Setenv("TC_DATABASE__MAX_CONNS", "25")
	cm, err := NewBuilder[Config]().AddEnv("TC_").Build(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, 25, cm.Get().Database.MaxConns)
}

func TestWatchAtomicReplacementAndRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"port":9000}`)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cm, err := NewBuilder[testConfig]().AddFile(path).WithWatch().Build(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	replace := func(data string) {
		temp := filepath.Join(dir, "next.json")
		writeConfig(t, temp, data)
		require.NoError(t, os.Rename(temp, path))
	}
	replace(`{"port":-1}`)
	select {
	case err := <-cm.Errors():
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not report validation failure")
	}
	require.Equal(t, 9000, cm.Get().Port)
	replace(`{}`)
	require.Eventually(t, func() bool { return cm.Get().Port == 8080 }, 3*time.Second, 10*time.Millisecond)
	snapshot := cm.Get()
	writeConfig(t, filepath.Join(dir, "unrelated.json"), `{}`)
	require.Never(t, func() bool { return cm.Get() != snapshot }, 150*time.Millisecond, 10*time.Millisecond)
	cancel()
	select {
	case _, open := <-cm.Errors():
		require.False(t, open)
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not stop on context cancellation")
	}
	require.NoError(t, cm.Close())
}

func TestConcurrentReloadAndClose(t *testing.T) {
	cm, err := Load[testConfig]()
	require.NoError(t, err)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				require.Equal(t, 8080, cm.Get().Port)
				err := cm.Reload()
				if err != nil && !errors.Is(err, ErrClosed) && !errors.Is(err, context.Canceled) {
					t.Error(err)
				}
			}
			require.NoError(t, cm.Close())
		}()
	}
	wg.Wait()
}

// subscriptionSource exercises registration callbacks and failure cleanup
// without depending on a remote SDK.
type subscriptionSource struct {
	notify func(error)
	stops  int
	data   string
	err    error
}

func (p *subscriptionSource) Read() (map[string]any, error) { return nil, nil }
func (p *subscriptionSource) ReadBytes() ([]byte, error)    { return []byte(p.data), nil }
func (p *subscriptionSource) Watch(notify func(error)) (func() error, error) {
	if p.err != nil {
		return nil, p.err
	}
	p.notify = notify
	notify(nil) // Synchronous notifications must not deadlock Build.
	return func() error { p.stops++; return nil }, nil
}
func TestSubscriptionCleanup(t *testing.T) {
	for _, setupFailure := range []bool{false, true} {
		p := &subscriptionSource{data: "{"}
		b := NewBuilder[testConfig]().AddProvider(p, json.Parser()).WithWatch()
		if setupFailure {
			b.AddProvider(&subscriptionSource{err: errors.New("cannot subscribe")}, json.Parser())
		}
		_, err := b.Build(t.Context())
		require.Error(t, err)
		require.Equal(t, 1, p.stops)
		p.notify(nil) // A late callback after failed Build remains harmless.
	}
	p := &subscriptionSource{data: `{}`}
	ctx, cancel := context.WithCancel(t.Context())
	cm, err := NewBuilder[testConfig]().AddProvider(p, json.Parser()).WithWatch().Build(ctx)
	require.NoError(t, err)
	cancel()
	require.NoError(t, cm.Close())
	require.Equal(t, 1, p.stops)
	p.notify(errors.New("late"))
	require.NoError(t, cm.Close())
	require.Equal(t, 1, p.stops)
}
