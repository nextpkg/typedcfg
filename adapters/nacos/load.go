package nacos

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/nextpkg/typedcfg"
)

// Options customizes Load. Zero values enable subscriptions with a 30-second
// reconciliation interval and a 5-second SDK timeout. NamespaceID and Group
// fall back to NACOS_NAMESPACE_ID and NACOS_GROUP, then public/DEFAULT_GROUP.
// Credentials are read once from ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET and
// NACOS_USERNAME/PASSWORD. Use Provider for caller-managed SDK settings.
type Options struct {
	NamespaceID  string
	Group        string
	Parser       koanf.Parser
	EnvPrefix    string // Optional business-configuration overrides; empty disables them.
	PollInterval time.Duration
	Timeout      time.Duration
	LogDir       string // Default: the user's cache directory, under typedcfg/nacos/log.
}

type managedClient interface {
	Client
	ListenConfig(vo.ConfigParam) error
	CancelListenConfig(vo.ConfigParam) error
	CloseClient()
}

// Manager owns its configuration subscription, SDK client, and temporary cache.
// Close is safe concurrently with Get, Reload, and other Close calls.
type Manager[T any] struct {
	config   *typedcfg.ConfigManager[T]
	client   managedClient
	key      subscriptionKey
	cacheDir string
	done     chan struct{}
	once     sync.Once
	closeErr error
}

// Get returns an independent deep copy of the latest valid configuration.
func (m *Manager[T]) Get() *T              { return m.config.Get() }
func (m *Manager[T]) Reload() error        { return m.config.Reload() }
func (m *Manager[T]) Errors() <-chan error { return m.config.Errors() }

// Close stops configuration updates before closing the owned SDK client and
// removing its temporary cache. SDK logs remain in LogDir for diagnostics.
func (m *Manager[T]) Close() error {
	if m == nil {
		return nil
	}
	m.once.Do(func() {
		m.closeErr = m.config.Close()
		m.client.CloseClient()
		releaseSubscription(m.key)
		m.closeErr = errors.Join(m.closeErr, os.RemoveAll(m.cacheDir))
		close(m.done)
	})
	return m.closeErr
}

// MustLoad loads and watches a Nacos configuration, panicking on startup errors.
// Address is a hostname/IP with an optional port (default 8848). Call Close on
// the returned manager when it is no longer needed.
func MustLoad[T any](address, dataID string, options ...Options) *Manager[T] {
	m, err := Load[T](context.Background(), address, dataID, options...)
	if err != nil {
		panic(err)
	}
	return m
}

// Load creates an owned SDK client and enables automatic updates. Canceling ctx
// also closes the manager. DataId selects JSON/YAML by extension; no extension
// defaults to YAML. At most one Options value is accepted. Environment values
// are literal: ${VAR} expansion and automatic credential rotation are not used.
func Load[T any](ctx context.Context, address, dataID string, options ...Options) (*Manager[T], error) {
	return loadWithFactory[T](ctx, address, dataID, func(p vo.NacosClientParam) (managedClient, error) { return clients.NewConfigClient(p) }, options...)
}

func loadWithFactory[T any](ctx context.Context, address, dataID string, factory func(vo.NacosClientParam) (managedClient, error), options ...Options) (*Manager[T], error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return nil, errors.New("configuration type must be a struct")
	}
	opts, server, clientConfig, parser, err := loadSettings(address, dataID, options)
	if err != nil {
		return nil, err
	}
	cacheDir, err := os.MkdirTemp("", "typedcfg-nacos-")
	if err != nil {
		return nil, err
	}
	clientConfig.CacheDir = cacheDir
	client, err := factory(vo.NacosClientParam{ServerConfigs: []constant.ServerConfig{server}, ClientConfig: &clientConfig})
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(cacheDir))
	}
	provider := Provider(client, dataID, opts.Group)
	key := subscriptionKey{client: client, dataID: dataID, group: provider.group}
	builder := typedcfg.NewBuilder[T]().AddProvider(provider, parser).WithWatch()
	if opts.PollInterval > 0 {
		builder.WithPollInterval(opts.PollInterval)
	}
	if opts.EnvPrefix != "" {
		builder.AddEnv(opts.EnvPrefix)
	}
	cm, err := builder.Build(ctx)
	if err != nil {
		client.CloseClient()
		releaseSubscription(key)
		return nil, errors.Join(err, os.RemoveAll(cacheDir))
	}
	m := &Manager[T]{config: cm, client: client, key: key, cacheDir: cacheDir, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = m.Close()
		case <-m.done:
		}
	}()
	return m, nil
}

func loadSettings(address, dataID string, options []Options) (Options, constant.ServerConfig, constant.ClientConfig, koanf.Parser, error) {
	var opts Options
	var server constant.ServerConfig
	var cfg constant.ClientConfig
	fail := func(message string) (Options, constant.ServerConfig, constant.ClientConfig, koanf.Parser, error) {
		return opts, server, cfg, nil, errors.New(message)
	}
	if len(options) > 1 {
		return fail("Load accepts at most one Options value")
	}
	if len(options) == 1 {
		opts = options[0]
	}
	if strings.TrimSpace(dataID) == "" {
		return fail("Nacos data ID must not be blank")
	}
	if opts.PollInterval < 0 || opts.Timeout < 0 {
		return fail("Nacos intervals must not be negative")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.Timeout < time.Millisecond {
		return fail("Nacos timeout must be at least one millisecond")
	}
	host := address
	port := uint64(8848)
	if net.ParseIP(address) == nil && strings.Contains(address, ":") {
		var portText string
		var err error
		host, portText, err = net.SplitHostPort(address)
		if err != nil {
			return fail("Nacos address must be a hostname/IP with optional port")
		}
		port, err = strconv.ParseUint(portText, 10, 16)
		if err != nil {
			return fail("invalid Nacos port")
		}
	}
	if host == "" || strings.ContainsAny(host, " /\\?#@[]\t\r\n") || port == 0 || port > 64535 {
		return fail("invalid Nacos host or port (gRPC uses port + 1000)")
	}
	server = constant.ServerConfig{IpAddr: host, Port: port}
	if opts.NamespaceID == "" {
		opts.NamespaceID = os.Getenv("NACOS_NAMESPACE_ID")
	}
	if opts.Group == "" {
		opts.Group = os.Getenv("NACOS_GROUP")
	}
	if opts.Group == "" {
		opts.Group = "DEFAULT_GROUP"
	}
	if strings.TrimSpace(opts.Group) == "" {
		return fail("Nacos group must not be blank")
	}
	parser := opts.Parser
	if parser == nil {
		switch strings.ToLower(filepath.Ext(dataID)) {
		case ".json":
			parser = json.Parser()
		case "", ".yaml", ".yml":
			parser = yaml.Parser()
		default:
			return fail("Nacos data ID needs a JSON/YAML extension or an explicit Parser")
		}
	} else {
		v := reflect.ValueOf(parser)
		switch v.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
			if v.IsNil() {
				return fail("Nacos parser is nil")
			}
		}
	}
	ak, sk := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"), os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	username, password := os.Getenv("NACOS_USERNAME"), os.Getenv("NACOS_PASSWORD")
	if (ak == "") != (sk == "") {
		return fail("ALIBABA_CLOUD_ACCESS_KEY_ID and ALIBABA_CLOUD_ACCESS_KEY_SECRET must both be set")
	}
	if (username == "") != (password == "") {
		return fail("NACOS_USERNAME and NACOS_PASSWORD must both be set")
	}
	if opts.LogDir == "" {
		cacheRoot, err := os.UserCacheDir()
		if err != nil {
			return opts, server, cfg, nil, err
		}
		opts.LogDir = filepath.Join(cacheRoot, "typedcfg", "nacos", "log")
	}
	cfg = constant.ClientConfig{NamespaceId: opts.NamespaceID, AccessKey: ak, SecretKey: sk, Username: username, Password: password, TimeoutMs: uint64(opts.Timeout / time.Millisecond), NotLoadCacheAtStart: true, DisableUseSnapShot: true, LogDir: opts.LogDir, LogLevel: "error", LogRollingConfig: &constant.ClientLogRollingConfig{MaxSize: 10, MaxBackups: 1, MaxAge: 7}}
	return opts, server, cfg, parser, nil
}
