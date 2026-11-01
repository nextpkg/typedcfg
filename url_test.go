package typedcfg

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knadh/koanf/parsers/json"
	"github.com/stretchr/testify/require"
)

func TestURLFormatsHeadersAndPriority(t *testing.T) {
	for _, extension := range []string{"json", "yaml", "yml", ""} {
		t.Run(extension, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer original" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if extension == "json" || extension == "" {
					_, _ = fmt.Fprint(w, `{"port":9090,"max_conns":25}`)
				} else {
					_, _ = fmt.Fprint(w, "port: 9090\nmax_conns: 25\n")
				}
			}))
			defer server.Close()
			options := URLOptions{Client: server.Client(), Headers: http.Header{"Authorization": []string{"Bearer original"}}}
			if extension == "" {
				options.Parser = json.Parser()
			}
			file := filepath.Join(t.TempDir(), "base.json")
			writeConfig(t, file, `{"port":8000,"max_conns":10}`)
			t.Setenv("URL_PORT", "9191")
			builder := NewBuilder[testConfig]().AddFile(file).AddURL(server.URL+"/config."+extension+"?version=1", options).AddEnv("URL_")
			options.Headers.Set("Authorization", "Bearer changed")
			cm, err := builder.Build(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, cm.Close()) })
			require.Equal(t, 9191, cm.Get().Port)
			require.Equal(t, 25, cm.Get().MaxConns)
		})
	}
}

func TestURLReloadRetainsSnapshotOnFailure(t *testing.T) {
	var status atomic.Int64
	status.Store(200)
	var body atomic.Value
	body.Store(`{"port":9000}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(status.Load()))
		_, _ = fmt.Fprint(w, body.Load().(string))
	}))
	defer server.Close()
	cm := NewBuilder[testConfig]().AddURL(server.URL+"/config.json?token=secret", URLOptions{MaxBytes: 64}).MustBuild()
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	for _, tc := range []struct {
		status int64
		body   string
	}{
		{500, "private error body"}, {200, "{"}, {200, `{"port":-1}`}, {200, strings.Repeat("x", 65)},
	} {
		old := cm.Get()
		status.Store(tc.status)
		body.Store(tc.body)
		err := cm.Reload()
		require.Error(t, err)
		require.NotContains(t, err.Error(), "token=secret")
		require.NotContains(t, err.Error(), "private error body")
		require.Same(t, old, cm.Get())
	}
	status.Store(200)
	body.Store(`{}`)
	require.NoError(t, cm.Reload())
	require.Equal(t, 8080, cm.Get().Port)
}

func TestURLTimeoutAndCloseCancellation(t *testing.T) {
	var block atomic.Bool
	started := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if block.Load() {
			started <- struct{}{}
			<-r.Context().Done()
			return
		}
		_, _ = fmt.Fprint(w, `{"port":9000}`)
	}))
	defer server.Close()
	block.Store(true)
	_, err := NewBuilder[testConfig]().AddURL(server.URL+"/config.json?token=secret", URLOptions{Timeout: 30 * time.Millisecond}).Build(t.Context())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotContains(t, err.Error(), "token=secret")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timeout request did not reach the server")
	}
	block.Store(false)
	cm, err := NewBuilder[testConfig]().AddURL(server.URL + "/config.json").Build(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm.Close()) })
	block.Store(true)
	result := make(chan error, 1)
	go func() { result <- cm.Reload() }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	require.NoError(t, cm.Close())
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel request")
	}
	require.Equal(t, 9000, cm.Get().Port)
}

func TestURLRedirectsAndValidation(t *testing.T) {
	var foreignCalls atomic.Int64
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignCalls.Add(1) }))
	defer foreign.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/same.json":
			http.Redirect(w, r, "/config.json", http.StatusFound)
		case "/foreign.json":
			http.Redirect(w, r, foreign.URL+"/config.json", http.StatusFound)
		default:
			_, _ = fmt.Fprint(w, `{"port":9090}`)
		}
	}))
	defer server.Close()
	cm, err := NewBuilder[testConfig]().AddURL(server.URL + "/same.json").Build(t.Context())
	require.NoError(t, err)
	require.Equal(t, 9090, cm.Get().Port)
	require.NoError(t, cm.Close())
	_, err = NewBuilder[testConfig]().AddURL(server.URL + "/foreign.json").Build(t.Context())
	require.ErrorContains(t, err, "changed scheme or host")
	require.Zero(t, foreignCalls.Load())
	for _, address := range []string{"file:///config.json", "/relative.json", "https://user:pass@example.com/config.json", "https://example.com/config", "https://example.com/config.json#fragment"} {
		_, err := NewBuilder[testConfig]().AddURL(address).Build(t.Context())
		require.Error(t, err)
	}
	_, err = NewBuilder[testConfig]().AddURL(server.URL+"/config.json", URLOptions{Timeout: -time.Second}).Build(t.Context())
	require.Error(t, err)
	_, err = NewBuilder[testConfig]().AddURL(server.URL + "/config.json").WithWatch().Build(t.Context())
	require.ErrorContains(t, err, "AddFile")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = NewBuilder[testConfig]().AddURL(server.URL + "/config.json").Build(ctx)
	require.True(t, errors.Is(err, context.Canceled))
}
