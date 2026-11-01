// Package nacos adapts a Nacos SDK configuration client, including Alibaba Cloud
// MSE Nacos, to typed configuration. Load owns the SDK lifecycle; Provider
// adapts a caller-owned SDK client.
package nacos

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// Client is the read-only part of the official Nacos configuration client.
// Configure its namespace, credentials, network timeout, and cache policy before
// passing it to Provider. GetConfig has no context cancellation parameter.
type Client interface {
	GetConfig(vo.ConfigParam) (string, error)
}

// Provider selects a configuration by DataId and Group. An empty group means
// DEFAULT_GROUP. Pass the result to Builder.AddProvider with a JSON/YAML parser.
// Each Build/Reload calls GetConfig. WithWatch subscribes through ListenConfig.
// Concurrent subscriptions through this adapter for the same client pointer,
// DataId and Group are rejected; SDK cancellation operates on that whole key.
func Provider(client Client, dataID, group string) *Config {
	if group == "" {
		group = "DEFAULT_GROUP"
	}
	return &Config{client: client, dataID: dataID, group: group}
}

// ErrAlreadyWatched means this adapter already owns a subscription for the key.
var ErrAlreadyWatched = errors.New("nacos configuration is already watched")

type subscriptionKey struct {
	client        Client
	dataID, group string
}

var subscriptions = struct {
	sync.Mutex
	active map[subscriptionKey]bool
}{active: make(map[subscriptionKey]bool)}

func reserveSubscription(key subscriptionKey) bool {
	subscriptions.Lock()
	defer subscriptions.Unlock()
	if subscriptions.active[key] {
		return false
	}
	subscriptions.active[key] = true
	return true
}

func releaseSubscription(key subscriptionKey) {
	subscriptions.Lock()
	defer subscriptions.Unlock()
	delete(subscriptions.active, key)
}

type Config struct {
	client   Client
	watchMu  sync.Mutex
	watching bool
	dataID   string
	group    string
}

func (p *Config) Read() (map[string]any, error) {
	return nil, errors.New("nacos source requires an explicit parser")
}

func (p *Config) ReadBytes() ([]byte, error) {
	if p == nil || p.client == nil || nilClient(p.client) {
		return nil, errors.New("nacos client is nil")
	}
	if strings.TrimSpace(p.dataID) == "" || strings.TrimSpace(p.group) == "" {
		return nil, errors.New("nacos data ID and group must not be blank")
	}
	content, err := p.client.GetConfig(vo.ConfigParam{DataId: p.dataID, Group: p.group})
	if err != nil {
		return nil, fmt.Errorf("read Nacos configuration: %w", err)
	}
	// Nacos may return empty content for a missing/deleted configuration. Do not
	// let a permissive parser turn that into a successful defaults-only reload.
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("nacos configuration is empty or missing")
	}
	return []byte(content), nil
}

func nilClient(client Client) bool {
	v := reflect.ValueOf(client)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return v.IsNil()
	}
	return false
}

// Watch subscribes to server changes. The manager re-reads and validates all
// sources on notification. Stopping removes the subscription but leaves the
// caller-owned SDK client open.
func (p *Config) Watch(notify func(error)) (func() error, error) {
	if p == nil || p.client == nil || nilClient(p.client) {
		return nil, errors.New("nacos client is nil")
	}
	c, ok := p.client.(interface {
		ListenConfig(vo.ConfigParam) error
		CancelListenConfig(vo.ConfigParam) error
	})
	if !ok {
		return nil, errors.New("nacos client does not support subscriptions")
	}
	if strings.TrimSpace(p.dataID) == "" || strings.TrimSpace(p.group) == "" {
		return nil, errors.New("nacos data ID and group must not be blank")
	}
	if notify == nil {
		return nil, errors.New("nacos notification callback is nil")
	}
	if reflect.ValueOf(p.client).Kind() != reflect.Pointer {
		return nil, errors.New("nacos subscription requires a pointer client identity")
	}
	p.watchMu.Lock()
	defer p.watchMu.Unlock()
	if p.watching {
		return nil, ErrAlreadyWatched
	}
	key := subscriptionKey{client: p.client, dataID: p.dataID, group: p.group}
	if !reserveSubscription(key) {
		return nil, ErrAlreadyWatched
	}
	param := vo.ConfigParam{DataId: p.dataID, Group: p.group, OnChange: func(_, _, _, _ string) { notify(nil) }}
	if err := c.ListenConfig(param); err != nil {
		releaseSubscription(key)
		return nil, fmt.Errorf("listen Nacos configuration: %w", err)
	}
	p.watching = true
	var once sync.Once
	var stopErr error
	return func() error {
		once.Do(func() {
			p.watchMu.Lock()
			defer p.watchMu.Unlock()
			stopErr = c.CancelListenConfig(vo.ConfigParam{DataId: p.dataID, Group: p.group})
			if stopErr == nil {
				p.watching = false
				releaseSubscription(key)
			}
		})
		return stopErr
	}, nil
}
