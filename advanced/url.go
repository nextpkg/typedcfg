package advanced

import (
	"context"
	"errors"
	"github.com/nextpkg/typedcfg/internal/httpconfig"
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
// It accepts at most one options value. WithWatch enables periodic reloads.
// Redirects must retain the original scheme and host (including port).
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
	reader *httpconfig.Reader
	ctx    context.Context
}

func newURLProvider(address string, options []URLOptions) (*urlProvider, koanf.Parser, error) {
	if len(options) > 1 {
		return nil, nil, errors.New("AddURL accepts at most one URLOptions value")
	}
	var opts URLOptions
	if len(options) == 1 {
		opts = options[0]
	}
	reader, err := httpconfig.New(address, httpconfig.Options{Timeout: opts.Timeout, MaxBytes: opts.MaxBytes, Headers: opts.Headers, Client: opts.Client})
	if err != nil {
		return nil, nil, err
	}
	u, _ := url.Parse(address)
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

	return &urlProvider{reader: reader}, parser, nil
}
func (p *urlProvider) Read() (map[string]any, error) {
	return nil, errors.New("URL source requires a parser")
}
func (p *urlProvider) ReadBytes() ([]byte, error) { return p.reader.Read(p.ctx) }
