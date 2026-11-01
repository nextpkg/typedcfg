package typedcfg

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type liveConfig struct {
	Port int    `json:"port" default:"8080"`
	Name string `json:"name" default:"service"`
}

func (c *liveConfig) Validate() error {
	if c.Port < 1 {
		return errors.New("port must be positive")
	}
	return nil
}
func TestFileLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cm, err := LoadContext[liveConfig](t.Context(), path, WithCreateIfMissing(), WithWatch(), WithPollInterval(10*time.Millisecond))
	require.NoError(t, err)
	defer func() {
		if err := cm.Close(); err != nil {
			t.Error(err)
		}
	}()
	require.Equal(t, 8080, cm.Get().Port)
	original := cm.Get()
	write := func(s string) {
		require.NoError(t, os.WriteFile(path+".new", []byte(s), 0600))
		require.NoError(t, os.Rename(path+".new", path))
	}
	write(`{"port":9090,"name":"next"}`)
	require.Eventually(t, func() bool { return cm.Get().Port == 9090 }, time.Second, time.Millisecond)
	require.Equal(t, 8080, original.Port)
	write(`{"port":0}`)
	select {
	case err := <-cm.Errors():
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("no error")
	}
	require.Equal(t, 9090, cm.Get().Port)
	write(`{"port":7070}`)
	require.Eventually(t, func() bool { return cm.Get().Port == 7070 }, time.Second, time.Millisecond)
	require.Equal(t, "service", cm.Get().Name)
	require.NoError(t, os.Remove(path))
	require.Error(t, cm.Reload())
	require.NoFileExists(t, path)
	require.Equal(t, 7070, cm.Get().Port)
	require.NoError(t, cm.Close())
	require.ErrorIs(t, cm.Reload(), ErrClosed)
}
func TestHTTPPollingAndCancellation(t *testing.T) {
	var body atomic.Value
	body.Store(`{"port":9000}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			w.WriteHeader(403)
			return
		}
		_, _ = fmt.Fprint(w, body.Load().(string))
	}))
	defer server.Close()
	cm, err := LoadContext[liveConfig](t.Context(), server.URL, WithHTTP(HTTPOptions{Headers: http.Header{"Authorization": []string{"Bearer test"}}}), WithWatch(), WithPollInterval(10*time.Millisecond))
	require.NoError(t, err)
	defer func() {
		if err := cm.Close(); err != nil {
			t.Error(err)
		}
	}()
	body.Store(`{"port":9001}`)
	require.Eventually(t, func() bool { return cm.Get().Port == 9001 }, time.Second, time.Millisecond)
	body.Store(`{"port":-1}`)
	require.Eventually(t, func() bool {
		select {
		case err := <-cm.Errors():
			require.Error(t, err)
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
	require.Equal(t, 9001, cm.Get().Port)
	ctx, cancel := context.WithCancel(t.Context())
	ready := make(chan struct{})
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(ready); <-r.Context().Done() }))
	defer blocked.Close()
	done := make(chan error, 1)
	go func() { _, err := LoadContext[liveConfig](ctx, blocked.URL); done <- err }()
	<-ready
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("HTTP did not cancel")
	}
}
func TestSubscriptionOwnership(t *testing.T) {
	var body atomic.Value
	body.Store(`{"port":9000}`)
	var notify func(error)
	var stops atomic.Int32
	subscription := WithSubscription(func(n func(error)) (func() error, error) {
		notify = n
		return func() error { stops.Add(1); return nil }, nil
	})
	cm, err := LoadSource[liveConfig](t.Context(), func(context.Context) ([]byte, error) { return []byte(body.Load().(string)), nil }, subscription, WithPollInterval(time.Hour))
	require.NoError(t, err)
	body.Store(`{"port":9002}`)
	notify(nil)
	require.Eventually(t, func() bool { return cm.Get().Port == 9002 }, time.Second, time.Millisecond)
	require.NoError(t, cm.Close())
	require.NoError(t, cm.Close())
	require.EqualValues(t, 1, stops.Load())
	body.Store("invalid")
	_, err = LoadSource[liveConfig](t.Context(), func(context.Context) ([]byte, error) { return []byte(body.Load().(string)), nil }, subscription)
	require.Error(t, err)
	require.EqualValues(t, 2, stops.Load())
}
func TestSimpleValidationContract(t *testing.T) {
	// validate tags are deliberately opt-in; custom Validate remains automatic.
	cm, err := Load[struct {
		Port int `validate:"min=1"`
	}]("")
	require.NoError(t, err)
	defer func() {
		if err := cm.Close(); err != nil {
			t.Error(err)
		}
	}()
	require.Zero(t, cm.Get().Port)
	_, err = Load[liveConfig]("", WithValidation(func(any) error { return errors.New("extra check") }))
	require.ErrorContains(t, err, "extra check")
	require.Panics(t, func() { MustLoad[liveConfig]("missing-file.json") })
}
