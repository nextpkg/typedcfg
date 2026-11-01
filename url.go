package typedcfg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

// URLOptions configures an HTTP source. Zero values use a 10-second timeout,
// a 1 MiB response limit, and the default HTTP transport. Parser is inferred from
// the URL path's .json/.yaml/.yml extension unless explicitly supplied.
type URLOptions struct {
	Timeout  time.Duration
	MaxBytes int64
	Headers  http.Header
	Client   *http.Client
	Parser   koanf.Parser
}

// AddURL adds an HTTP(S) GET source, fetched during Build and each Reload.
// It accepts at most one options value. It does not poll, watch, or create remote
// files. Redirects must retain the original scheme and host (including port).
func (b *Builder[T]) AddURL(address string, options ...URLOptions) *Builder[T] {
	p, parser, err := newURLProvider(address, options)
	if err != nil {
		b.err = err
		return b
	}
	b.sources = append(b.sources, source{provider: p, parser: parser})
	return b
}

type urlProvider struct {
	address  string
	client   *http.Client
	headers  http.Header
	timeout  time.Duration
	maxBytes int64
	ctx      context.Context
}

func newURLProvider(address string, options []URLOptions) (*urlProvider, koanf.Parser, error) {
	if len(options) > 1 {
		return nil, nil, errors.New("AddURL accepts at most one URLOptions value")
	}
	u, err := url.Parse(address)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, nil, errors.New("configuration URL must be an absolute HTTP(S) URL without userinfo or fragment")
	}
	var opts URLOptions
	if len(options) == 1 {
		opts = options[0]
	}
	if opts.Timeout < 0 || opts.MaxBytes < 0 || opts.MaxBytes == math.MaxInt64 {
		return nil, nil, errors.New("URL timeout and response limit must be positive or zero for defaults")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 1 << 20
	}
	parser := opts.Parser
	if parser == nil {
		switch strings.ToLower(path.Ext(u.Path)) {
		case ".json":
			parser = json.Parser()
		case ".yaml", ".yml":
			parser = yaml.Parser()
		default:
			return nil, nil, errors.New("URL path needs a .json/.yaml/.yml extension or an explicit Parser")
		}
	} else if isNil(parser) {
		return nil, nil, errors.New("URL parser is nil")
	}
	client := *http.DefaultClient
	if opts.Client != nil {
		client = *opts.Client
	}
	redirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many configuration redirects")
		}
		if req.URL.Scheme != u.Scheme || !strings.EqualFold(req.URL.Host, u.Host) {
			return errors.New("configuration redirect changed scheme or host")
		}
		if redirect != nil {
			return redirect(req, via)
		}
		return nil
	}
	return &urlProvider{address: address, client: &client, headers: opts.Headers.Clone(), timeout: opts.Timeout, maxBytes: opts.MaxBytes}, parser, nil
}

func (p *urlProvider) Read() (map[string]any, error) {
	return nil, errors.New("URL source requires a parser")
}

func (p *urlProvider) ReadBytes() (data []byte, err error) {
	ctx := p.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.address, nil)
	if err != nil {
		return nil, errors.New("invalid configuration request")
	}
	req.Header = p.headers.Clone()
	response, err := p.client.Do(req)
	if err != nil {
		// net/http wraps errors with the full URL, which may contain signed query values.
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			err = requestErr.Err
		}
		return nil, fmt.Errorf("fetch configuration: %w", err)
	}
	defer func() { err = errors.Join(err, response.Body.Close()) }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("configuration server returned HTTP %d", response.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, p.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read configuration response: %w", err)
	}
	if int64(len(data)) > p.maxBytes {
		return nil, fmt.Errorf("configuration response exceeds %d bytes", p.maxBytes)
	}
	return data, nil
}
