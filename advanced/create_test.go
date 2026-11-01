package advanced

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateMissingDefaults(t *testing.T) {
	type Config struct {
		Server struct {
			Port    int           `json:"listen_port" default:"8080"`
			Timeout time.Duration `json:"timeout" default:"30s"`
		} `json:"server"`
		Enabled bool     `json:"enabled" default:"true"`
		Tags    []string `json:"tags" default:"web,api"`
	}
	for _, extension := range []string{"json", "yaml", "yml"} {
		t.Run(extension, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "new", "config."+extension)
			t.Setenv("GEN_SERVER__LISTEN_PORT", "9090")
			cm := NewBuilder[Config]().AddFile(path).AddEnv("GEN_").WithCreateIfMissing().MustBuild()
			t.Cleanup(func() { require.NoError(t, cm.Close()) })
			require.Equal(t, 9090, cm.Get().Server.Port)
			onDisk := MustLoad[Config](path)
			t.Cleanup(func() { require.NoError(t, onDisk.Close()) })
			require.Equal(t, 8080, onDisk.Get().Server.Port)
			require.Equal(t, 30*time.Second, onDisk.Get().Server.Timeout)
			require.True(t, onDisk.Get().Enabled)
			require.Equal(t, []string{"web", "api"}, onDisk.Get().Tags)
			contents, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Contains(t, string(contents), "listen_port")
			require.NotContains(t, string(contents), "9090")
			entries, err := os.ReadDir(filepath.Dir(path))
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

func TestCreatePreservesExistingFiles(t *testing.T) {
	for _, contents := range []string{`{"port":9000}`, `{invalid`} {
		t.Run(contents, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			writeConfig(t, path, contents)
			cm, err := NewBuilder[testConfig]().AddFile(path).WithCreateIfMissing().Build(t.Context())
			if contents == `{invalid` {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, 9000, cm.Get().Port)
				require.NoError(t, cm.Close())
			}
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, contents, string(data))
		})
	}
}

func TestCreateAndWatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "config.yaml")
	cm, err := NewBuilder[testConfig]().AddFile(path).WithCreateIfMissing().WithWatch().Build(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	old := cm.Get()
	require.Equal(t, 8080, old.Port)
	writeConfig(t, path, "port: 9090\n")
	require.Eventually(t, func() bool { return cm.Get().Port == 9090 }, 3*time.Second, 10*time.Millisecond)
	require.Equal(t, 8080, old.Port)
	require.NoError(t, os.Remove(path))
	select {
	case err := <-cm.Errors():
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("deleted configuration did not report an error")
	}
	require.NoFileExists(t, path)
	require.Equal(t, 9090, cm.Get().Port)
}

func TestConcurrentCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm, err := NewBuilder[testConfig]().AddFile(path).WithCreateIfMissing().Build(t.Context())
			if err == nil {
				err = cm.Close()
			}
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	cm := MustLoad[testConfig](path)
	require.Equal(t, 8080, cm.Get().Port)
	require.NoError(t, cm.Close())
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestCreationErrorsAndAlternateTag(t *testing.T) {
	type Invalid struct {
		Port int `json:"port" default:"not-an-int"`
	}
	path := filepath.Join(t.TempDir(), "config.json")
	_, err := NewBuilder[Invalid]().AddFile(path).WithCreateIfMissing().Build(t.Context())
	require.Error(t, err)
	require.NoFileExists(t, path)
	blocked := filepath.Join(t.TempDir(), "file")
	writeConfig(t, blocked, "existing")
	_, err = NewBuilder[testConfig]().AddFile(filepath.Join(blocked, "config.json")).WithCreateIfMissing().Build(t.Context())
	require.Error(t, err)
	type Config struct {
		MaxConns int `koanf:"max_conns" default:"25"`
	}
	cm, err := NewBuilder[Config]().WithTag("koanf").AddFile(path).WithCreateIfMissing().Build(t.Context())
	require.NoError(t, err)
	require.Equal(t, 25, cm.Get().MaxConns)
	require.NoError(t, cm.Close())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "max_conns")
}
