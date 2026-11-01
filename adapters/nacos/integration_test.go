package nacos_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/knadh/koanf/parsers/json"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/nextpkg/typedcfg"
	"github.com/nextpkg/typedcfg/adapters/nacos"
	"github.com/stretchr/testify/require"
)

// TestServerIntegration publishes and removes its own unique configuration on
// an explicitly supplied local Nacos test server (no cloud credentials used).
func TestServerIntegration(t *testing.T) {
	portText := os.Getenv("TYPEDCFG_NACOS_TEST_PORT")
	if portText == "" {
		t.Skip("set TYPEDCFG_NACOS_TEST_PORT for a local disposable Nacos server")
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	require.NoError(t, err)
	dir := t.TempDir()
	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ServerConfigs: []constant.ServerConfig{{IpAddr: "127.0.0.1", Port: port}},
		ClientConfig: &constant.ClientConfig{
			TimeoutMs: 5000, NotLoadCacheAtStart: true, DisableUseSnapShot: true,
			LogDir: filepath.Join(dir, "logs"), CacheDir: filepath.Join(dir, "cache"), LogLevel: "error",
		},
	})
	require.NoError(t, err)
	t.Cleanup(client.CloseClient)
	param := vo.ConfigParam{DataId: fmt.Sprintf("typedcfg-%d.json", time.Now().UnixNano()), Group: "TYPEDCFG_TEST", Content: `{"port":9000,"name":"remote"}`, Type: "json"}
	t.Cleanup(func() {
		ok, err := client.DeleteConfig(param)
		require.NoError(t, err)
		require.True(t, ok)
	})
	ok, err := client.PublishConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	cm, err := typedcfg.NewBuilder[config]().AddProvider(nacos.Provider(client, param.DataId, param.Group), json.Parser()).Build(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, config{9000, "remote"}, *cm.Get())
	param.Content = `{"port":9001}`
	ok, err = client.PublishConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, cm.Reload())
	require.Equal(t, config{9001, "app"}, *cm.Get())
	old := cm.Get()
	param.Content = `{"port":-1}`
	ok, err = client.PublishConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	require.Error(t, cm.Reload())
	require.Same(t, old, cm.Get())
	ok, err = client.DeleteConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	require.Error(t, cm.Reload())
	require.Same(t, old, cm.Get())
}
