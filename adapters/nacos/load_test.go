package nacos

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/nextpkg/typedcfg"
	"github.com/stretchr/testify/require"
)

type ownedFake struct {
	mu               sync.Mutex
	content          string
	callback         func(string, string, string, string)
	closed, canceled int
}

func (c *ownedFake) GetConfig(vo.ConfigParam) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.content, nil
}
func (c *ownedFake) ListenConfig(p vo.ConfigParam) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callback = p.OnChange
	return nil
}
func (c *ownedFake) CancelListenConfig(vo.ConfigParam) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.canceled++
	c.callback = nil
	return nil
}
func (c *ownedFake) CloseClient() { c.mu.Lock(); defer c.mu.Unlock(); c.closed++ }
func (c *ownedFake) publish(content string) {
	c.mu.Lock()
	c.content = content
	callback := c.callback
	c.mu.Unlock()
	if callback != nil {
		callback("", "", "", content)
	}
}
func clearLoadEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"ALIBABA_CLOUD_ACCESS_KEY_ID", "ALIBABA_CLOUD_ACCESS_KEY_SECRET", "NACOS_USERNAME", "NACOS_PASSWORD", "NACOS_NAMESPACE_ID", "NACOS_GROUP"} {
		t.Setenv(key, "")
	}
}

type loadConfig struct {
	Port int `json:"port" default:"8080" validate:"min=1"`
}

func TestLoadOwnedLifecycle(t *testing.T) {
	clearLoadEnv(t)
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "test-ak")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "test-sk")
	t.Setenv("NACOS_NAMESPACE_ID", "test-namespace")
	t.Setenv("NACOS_GROUP", "test-group")
	client := &ownedFake{content: "port: 9000"}
	var param vo.NacosClientParam
	factory := func(p vo.NacosClientParam) (managedClient, error) { param = p; return client, nil }
	cm, err := loadWithFactory[loadConfig](t.Context(), "127.0.0.1:18848", "app.yaml", factory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, "test-ak", param.ClientConfig.AccessKey)
	require.Equal(t, "test-sk", param.ClientConfig.SecretKey)
	require.Equal(t, "test-namespace", param.ClientConfig.NamespaceId)
	require.Equal(t, "test-group", cm.key.group)
	require.True(t, param.ClientConfig.DisableUseSnapShot)
	require.True(t, param.ClientConfig.NotLoadCacheAtStart)
	require.EqualValues(t, 5000, param.ClientConfig.TimeoutMs)
	require.EqualValues(t, 18848, param.ServerConfigs[0].Port)
	require.Equal(t, 9000, cm.Get().Port)
	require.DirExists(t, param.ClientConfig.CacheDir)
	client.publish("port: 9001")
	require.Eventually(t, func() bool { return cm.Get().Port == 9001 }, time.Second, time.Millisecond)
	client.publish("port: -1")
	select {
	case err := <-cm.Errors():
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("missing validation error")
	}
	require.Equal(t, 9001, cm.Get().Port)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := cm.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	require.Equal(t, 1, client.closed)
	require.Equal(t, 1, client.canceled)
	require.NoDirExists(t, param.ClientConfig.CacheDir)
	require.ErrorIs(t, cm.Reload(), typedcfg.ErrClosed)
}

func TestLoadCancellationAndEnvOverrides(t *testing.T) {
	clearLoadEnv(t)
	t.Setenv("EXAMPLE_PORT", "9100")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	client := &ownedFake{content: `{"port":9000}`}
	cm, err := loadWithFactory[loadConfig](ctx, "localhost", "app.json", func(vo.NacosClientParam) (managedClient, error) { return client, nil }, Options{EnvPrefix: "EXAMPLE_"})
	require.NoError(t, err)
	require.Equal(t, 9100, cm.Get().Port)
	cancel()
	select {
	case <-cm.done:
	case <-time.After(time.Second):
		t.Fatal("context did not close client")
	}
	require.NoError(t, cm.Close())
	require.Equal(t, 1, client.closed)
	require.NoDirExists(t, cm.cacheDir)
}

func TestLoadFailuresCleanOwnedResources(t *testing.T) {
	clearLoadEnv(t)
	for _, factoryFailure := range []bool{false, true} {
		client := &ownedFake{content: "{"}
		var cacheDir string
		_, err := loadWithFactory[loadConfig](t.Context(), "localhost", "app.json", func(p vo.NacosClientParam) (managedClient, error) {
			cacheDir = p.ClientConfig.CacheDir
			if factoryFailure {
				return nil, errors.New("cannot create client")
			}
			return client, nil
		})
		require.Error(t, err)
		require.NoDirExists(t, cacheDir)
		if !factoryFailure {
			require.Equal(t, 1, client.closed)
			require.Equal(t, 1, client.canceled)
		}
	}
}

func TestLoadSettingsAndEarlyErrors(t *testing.T) {
	clearLoadEnv(t)
	for _, address := range []string{"", "http://localhost:8848", "user:pass@localhost", "host/path", "host:0", "host:65000"} {
		_, _, _, _, err := loadSettings(address, "app.yaml", nil)
		require.Error(t, err)
	}
	_, server, _, parser, err := loadSettings("[::1]:18848", "app", nil)
	require.NoError(t, err)
	require.Equal(t, "::1", server.IpAddr)
	require.NotNil(t, parser)
	_, _, _, _, err = loadSettings("localhost", "app.properties", nil)
	require.Error(t, err)
	_, _, _, _, err = loadSettings("localhost", "app.yaml", []Options{{Timeout: time.Nanosecond}})
	require.Error(t, err)
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "secret-not-in-error")
	_, err = Load[loadConfig](t.Context(), "localhost", "app.yaml")
	require.ErrorContains(t, err, "must both be set")
	require.NotContains(t, err.Error(), "secret-not-in-error")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "test-sk")
	t.Setenv("NACOS_USERNAME", "test-user")
	_, _, _, _, err = loadSettings("localhost", "app.yaml", nil)
	require.Error(t, err)
	t.Setenv("NACOS_PASSWORD", "test-password")
	_, _, cfg, _, err := loadSettings("localhost", "app.yaml", []Options{{Group: "explicit", NamespaceID: "explicit", LogDir: filepath.Join(t.TempDir(), "log")}})
	require.NoError(t, err)
	require.Equal(t, "test-user", cfg.Username)
	require.Equal(t, "test-password", cfg.Password)
	require.Panics(t, func() { MustLoad[loadConfig]("localhost", "app.unsupported") })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = Load[loadConfig](ctx, "localhost", "app.yaml")
	require.ErrorIs(t, err, context.Canceled)
}
