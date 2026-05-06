package datadash

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// HTTPSource polls an HTTP endpoint at a fixed interval and exposes
// the responses as an io.Reader stream. Each response body becomes one
// line in the stream (newline-appended), so a JSON-emitting endpoint
// looks identical to a streaming NDJSON pipe.
//
// Usage:
//
//	src := datadash.NewHTTPSource(ctx, "https://api.example.com/metrics",
//	    datadash.HTTPInterval(5*time.Second))
//	reader := datadash.NewReader(src, datadash.WithFormat(datadash.FormatJSON))
type HTTPSource struct {
	url      string
	interval time.Duration
	client   *http.Client
	headers  map[string]string

	pr *io.PipeReader
	pw *io.PipeWriter
}

// HTTPOption configures an HTTPSource.
type HTTPOption func(*HTTPSource)

// HTTPInterval sets the polling interval. Defaults to 5 seconds.
func HTTPInterval(d time.Duration) HTTPOption {
	return func(s *HTTPSource) { s.interval = d }
}

// HTTPHeader adds a request header (call multiple times for multiple headers).
// Useful for Authorization tokens, content negotiation, etc.
func HTTPHeader(key, value string) HTTPOption {
	return func(s *HTTPSource) {
		if s.headers == nil {
			s.headers = make(map[string]string)
		}
		s.headers[key] = value
	}
}

// HTTPTimeout sets the per-request timeout. Defaults to 10 seconds.
func HTTPTimeout(d time.Duration) HTTPOption {
	return func(s *HTTPSource) {
		s.client = &http.Client{Timeout: d}
	}
}

// NewHTTPSource starts a polling goroutine and returns a source that
// can be passed to NewReader. The poll loop terminates when ctx is
// cancelled or the reader side is closed.
func NewHTTPSource(ctx context.Context, url string, opts ...HTTPOption) *HTTPSource {
	pr, pw := io.Pipe()
	s := &HTTPSource{
		url:      url,
		interval: 5 * time.Second,
		client:   &http.Client{Timeout: 10 * time.Second},
		pr:       pr,
		pw:       pw,
	}
	for _, o := range opts {
		o(s)
	}
	go s.run(ctx)
	return s
}

// Read implements io.Reader.
func (s *HTTPSource) Read(b []byte) (int, error) {
	return s.pr.Read(b)
}

// Close stops the polling goroutine.
func (s *HTTPSource) Close() error {
	return s.pr.Close()
}

func (s *HTTPSource) run(ctx context.Context) {
	defer s.pw.Close()

	// Fire once immediately so the user sees data without waiting a
	// full interval.
	s.poll(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.poll(ctx) {
				return
			}
		}
	}
}

// poll performs one request and writes the body as a single line.
// Returns false on a fatal write error (consumer closed the reader).
func (s *HTTPSource) poll(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
	if err != nil {
		slog.Debug("http source: build request", "err", err)
		return true
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Debug("http source: request failed", "url", s.url, "err", err)
		return true // transient; keep polling
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("http source: error status", "url", s.url, "status", resp.StatusCode)
		// Drain body but emit nothing.
		_, _ = io.Copy(io.Discard, resp.Body)
		return true
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug("http source: read body", "err", err)
		return true
	}

	// Some endpoints return arrays-of-objects or pretty-printed JSON.
	// Squash everything onto a single line so NDJSON parsing works.
	body = squashJSON(body)
	if len(body) == 0 {
		return true
	}

	if _, err := s.pw.Write(append(body, '\n')); err != nil {
		// Reader was closed, we're done.
		return false
	}
	return true
}

// squashJSON removes newlines from a JSON document so it fits on one
// line. It does *not* re-format — it just strips '\n' and '\r' that
// aren't inside string literals. This is sufficient for the common case
// of pretty-printed JSON from an API endpoint.
func squashJSON(b []byte) []byte {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return b
	}

	out := make([]byte, 0, len(b))
	inStr := false
	escape := false

	for _, c := range b {
		switch {
		case escape:
			escape = false
			out = append(out, c)
		case c == '\\' && inStr:
			escape = true
			out = append(out, c)
		case c == '"':
			inStr = !inStr
			out = append(out, c)
		case (c == '\n' || c == '\r') && !inStr:
			// skip
		default:
			out = append(out, c)
		}
	}
	return out
}

// FetchOnce performs a single HTTP GET and returns the body. Useful
// for the `--url` mode when polling isn't needed (e.g. one-shot snapshot).
func FetchOnce(ctx context.Context, url string, opts ...HTTPOption) ([]byte, error) {
	s := &HTTPSource{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		o(s)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
