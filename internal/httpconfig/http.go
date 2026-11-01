package httpconfig

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Options struct {
	Timeout  time.Duration
	MaxBytes int64
	Headers  http.Header
	Client   *http.Client
}
type Reader struct {
	address  string
	client   *http.Client
	headers  http.Header
	timeout  time.Duration
	maxBytes int64
}

func New(address string, opts Options) (*Reader, error) {
	u, err := url.Parse(address)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("configuration URL must be an absolute HTTP(S) URL without userinfo or fragment")
	}
	if opts.Timeout < 0 || opts.MaxBytes < 0 || opts.MaxBytes == math.MaxInt64 {
		return nil, errors.New("URL timeout and response limit must be positive or zero for defaults")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 1 << 20
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
	return &Reader{address: address, client: &client, headers: opts.Headers.Clone(), timeout: opts.Timeout, maxBytes: opts.MaxBytes}, nil
}
func (p *Reader) Read(ctx context.Context) (data []byte, err error) {
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
