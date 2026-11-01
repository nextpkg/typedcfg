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
	"github.com/nacos-group/nacos-sdk-go/v2/common/logger"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/nextpkg/typedcfg"
	"github.com/nextpkg/typedcfg/adapters/nacos"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestServerIntegration publishes and removes a unique configuration on an
// explicitly supplied test instance. The default host is localhost; MSE tests
// can supply a hostname, namespace, and RAM credentials through the environment.
func TestServerIntegration(t *testing.T) {
	portText := os.Getenv("TYPEDCFG_NACOS_TEST_PORT")
	if portText == "" {
		t.Skip("set TYPEDCFG_NACOS_TEST_PORT for a disposable Nacos test instance")
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	require.NoError(t, err)
	dir := t.TempDir()
	host := os.Getenv("TYPEDCFG_NACOS_TEST_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ServerConfigs: []constant.ServerConfig{{IpAddr: host, Port: port}},
		ClientConfig: &constant.ClientConfig{
			NamespaceId: os.Getenv("TYPEDCFG_NACOS_TEST_NAMESPACE"),
			AccessKey:   os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"),
			SecretKey:   os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET"),
			TimeoutMs:   5000, NotLoadCacheAtStart: true, DisableUseSnapShot: true,
			LogDir: filepath.Join(dir, "logs"), CacheDir: filepath.Join(dir, "cache"), LogLevel: "error",
		},
	})
	require.NoError(t, err)
	// Client initialization installs the SDK's process-global file logger.
	// Replace it before requests so shutdown logging cannot recreate TempDir.
	logger.SetLogger(zap.NewNop().Sugar())
	t.Cleanup(client.CloseClient)
	param := vo.ConfigParam{DataId: fmt.Sprintf("typedcfg-%d.json", time.Now().UnixNano()), Group: "TYPEDCFG_TEST", Content: `{"port":9000,"name":"remote"}`, Type: "json"}
	// Publish acknowledges persistence before every server read necessarily sees
	// the new value. Bound propagation waiting separately from provider assertions.
	waitContent := func(want string) {
		t.Helper()
		require.Eventually(t, func() bool {
			got, err := client.GetConfig(param)
			return err == nil && got == want
		}, 15*time.Second, 200*time.Millisecond, "configuration did not become visible")
	}
	t.Cleanup(func() {
		ok, err := client.DeleteConfig(param)
		require.NoError(t, err)
		require.True(t, ok)
	})
	ok, err := client.PublishConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	waitContent(param.Content)
	cm, err := typedcfg.NewBuilder[config]().AddProvider(nacos.Provider(client, param.DataId, param.Group), json.Parser()).WithWatch().WithPollInterval(500 * time.Millisecond).Build(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, config{9000, "remote"}, *cm.Get())
	param.Content = `{"port":9001}`
	ok, err = client.PublishConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	waitContent(param.Content)
	require.Eventually(t, func() bool { return *cm.Get() == (config{9001, "app"}) }, 15*time.Second, 100*time.Millisecond, "subscription did not update snapshot")
	old := cm.Get()
	param.Content = `{"port":-1}`
	ok, err = client.PublishConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	waitContent(param.Content)
	select {
	case err := <-cm.Errors():
		require.Error(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("subscription did not report invalid/deleted configuration")
	}
	require.Equal(t, *old, *cm.Get())
	ok, err = client.DeleteConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	require.Eventually(t, func() bool {
		page, err := client.SearchConfig(vo.SearchConfigParam{
			Search: "accurate", DataId: param.DataId, Group: param.Group, PageNo: 1, PageSize: 10,
		})
		return err == nil && page != nil && page.TotalCount == 0
	}, 15*time.Second, 200*time.Millisecond, "deleted configuration is still listed")
	// Periodic reconciliation covers SDK notifications missed on deletion.
	select {
	case err := <-cm.Errors():
		require.Error(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("deletion did not report an asynchronous error")
	}
	require.Equal(t, *old, *cm.Get())
	param.Content = `{"port":9002}`
	ok, err = client.PublishConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	require.Eventually(t, func() bool { return cm.Get().Port == 9002 }, 15*time.Second, 100*time.Millisecond, "subscription did not recover after recreation")
	require.NoError(t, cm.Close())
	param.Content = `{"port":9003}`
	ok, err = client.PublishConfig(param)
	require.NoError(t, err)
	require.True(t, ok)
	waitContent(param.Content)
	require.Equal(t, 9002, cm.Get().Port)

}
