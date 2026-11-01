package nacos_test

import (
	"context"
	"errors"
	"testing"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/nextpkg/typedcfg"
	"github.com/nextpkg/typedcfg/adapters/nacos"
	"github.com/stretchr/testify/require"
)

type client struct {
	content string
	err     error
	param   vo.ConfigParam
}

func (c *client) GetConfig(param vo.ConfigParam) (string, error) {
	c.param = param
	return c.content, c.err
}

type config struct {
	Port int    `json:"port" default:"8080" validate:"min=1"`
	Name string `json:"name" default:"app"`
}

func TestReload(t *testing.T) {
	c := &client{content: `{"port":9000,"name":"remote"}`}
	cm, err := typedcfg.NewBuilder[config]().AddProvider(nacos.Provider(c, "app.json", ""), json.Parser()).Build(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, vo.ConfigParam{DataId: "app.json", Group: "DEFAULT_GROUP"}, c.param)
	require.Equal(t, config{9000, "remote"}, *cm.Get())
	old := cm.Get()
	readErr := errors.New("connection unavailable")
	for _, tc := range []struct {
		name, content string
		err           error
	}{
		{"network", "", readErr},
		{"empty", "", nil},
		{"whitespace", " \n", nil},
		{"parse", "broken", nil},
		{"validation", `{"port":-1}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c.content, c.err = tc.content, tc.err
			err := cm.Reload()
			require.Error(t, err)
			if tc.err != nil {
				require.ErrorIs(t, err, readErr)
			}
			require.Same(t, old, cm.Get())
		})
	}
	c.content, c.err = `{"port":9001}`, nil
	require.NoError(t, cm.Reload())
	require.Equal(t, config{9001, "app"}, *cm.Get())
	require.Equal(t, config{9000, "remote"}, *old)
}

func TestYAMLAndEnvironmentPriority(t *testing.T) {
	t.Setenv("NACOS_TEST_PORT", "9002")
	c := &client{content: "port: 9000\nname: remote\n"}
	cm, err := typedcfg.NewBuilder[config]().AddProvider(nacos.Provider(c, "app.yaml", "production"), yaml.Parser()).AddEnv("NACOS_TEST_").Build(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	require.Equal(t, "production", c.param.Group)
	require.Equal(t, config{9002, "remote"}, *cm.Get())
}

func TestInvalidProvider(t *testing.T) {
	var typedNil *client
	for _, p := range []*nacos.Config{
		nil, nacos.Provider(nil, "a", ""), nacos.Provider(typedNil, "a", ""),
		nacos.Provider(&client{}, " ", ""), nacos.Provider(&client{}, "a", " "),
	} {
		_, err := p.ReadBytes()
		require.Error(t, err)
	}
	_, err := nacos.Provider(&client{}, "a", "").Read()
	require.Error(t, err)
}

type listenerClient struct {
	client
	listenErr error
	cancels   int
	callback  func(string, string, string, string)
}

func (c *listenerClient) ListenConfig(p vo.ConfigParam) error {
	c.callback = p.OnChange
	return c.listenErr
}
func (c *listenerClient) CancelListenConfig(vo.ConfigParam) error { c.cancels++; return nil }

func TestSubscriptionLifecycle(t *testing.T) {
	c := &listenerClient{client: client{content: `{}`}}
	p := nacos.Provider(c, "app.json", "")
	events := 0
	stop, err := p.Watch(func(err error) { require.NoError(t, err); events++ })
	require.NoError(t, err)
	c.callback("", "", "", "")
	require.Equal(t, 1, events)
	_, err = p.Watch(func(error) {})
	require.ErrorContains(t, err, "already watched")
	require.NoError(t, stop())
	require.NoError(t, stop())
	require.Equal(t, 1, c.cancels)
	c.listenErr = errors.New("unavailable")
	_, err = p.Watch(func(error) {})
	require.ErrorIs(t, err, c.listenErr)
	_, err = nacos.Provider(&client{}, "app.json", "").Watch(func(error) {})
	require.ErrorContains(t, err, "does not support")
}
