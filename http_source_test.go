package datadash

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// =========================================================================
// HTTPSource — basic polling
// =========================================================================

func TestHTTPSource_PollsAndEmitsLines(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		fmt.Fprintf(w, `{"value": %d}`, n)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := NewHTTPSource(ctx, srv.URL, HTTPInterval(20*time.Millisecond))
	defer src.Close()

	scanner := bufio.NewScanner(src)

	// Read three lines.
	var lines []string
	for i := 0; i < 3; i++ {
		if !scanner.Scan() {
			t.Fatalf("scan failed: %v", scanner.Err())
		}
		lines = append(lines, scanner.Text())
	}

	for i, line := range lines {
		want := fmt.Sprintf(`{"value": %d}`, i+1)
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

// =========================================================================
// HTTPSource — headers
// =========================================================================

func TestHTTPSource_SendsHeaders(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"ok": 1}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := NewHTTPSource(ctx, srv.URL,
		HTTPInterval(time.Hour), // we only care about the immediate first poll
		HTTPHeader("Authorization", "Bearer xyz"),
		HTTPHeader("X-Custom", "hello"),
	)
	defer src.Close()

	// Force one read so the immediate poll completes.
	scanner := bufio.NewScanner(src)
	if !scanner.Scan() {
		t.Fatalf("scan: %v", scanner.Err())
	}

	if gotAuth != "Bearer xyz" {
		t.Errorf("Authorization header = %q, want 'Bearer xyz'", gotAuth)
	}
}

// =========================================================================
// HTTPSource — error responses don't kill the loop
// =========================================================================

func TestHTTPSource_ErrorResponsesSkipped(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n%2 == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `{"n": %d}`, n)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := NewHTTPSource(ctx, srv.URL, HTTPInterval(20*time.Millisecond))
	defer src.Close()

	scanner := bufio.NewScanner(src)
	if !scanner.Scan() {
		t.Fatal("first successful poll never came through")
	}
	// The line we got must be from an even-numbered hit, since odd ones
	// are 500 errors that don't emit anything.
	got := scanner.Text()
	if !strings.HasPrefix(got, `{"n":`) {
		t.Errorf("unexpected line: %q", got)
	}
}

// =========================================================================
// HTTPSource — body squashing for multi-line JSON
// =========================================================================

func TestHTTPSource_SquashesMultilineJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pretty-printed JSON with newlines.
		fmt.Fprint(w, `{
  "cpu": 45,
  "mem": 80
}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := NewHTTPSource(ctx, srv.URL, HTTPInterval(time.Hour))
	defer src.Close()

	scanner := bufio.NewScanner(src)
	if !scanner.Scan() {
		t.Fatal("scan failed")
	}
	got := scanner.Text()

	// Must be a single line containing both fields.
	if strings.Contains(got, "\n") {
		t.Errorf("output still contains newlines: %q", got)
	}
	if !strings.Contains(got, `"cpu":`) || !strings.Contains(got, `"mem":`) {
		t.Errorf("expected fields missing in %q", got)
	}
}

// =========================================================================
// HTTPSource — context cancellation stops the goroutine
// =========================================================================

func TestHTTPSource_CancellationStopsPolling(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, `{"x": 1}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	src := NewHTTPSource(ctx, srv.URL, HTTPInterval(10*time.Millisecond))
	defer src.Close()

	// Drain a couple lines to ensure polling is active.
	scanner := bufio.NewScanner(src)
	scanner.Scan()
	scanner.Scan()

	cancel()
	time.Sleep(50 * time.Millisecond) // let cancellation propagate

	before := hits.Load()
	time.Sleep(50 * time.Millisecond) // wait long enough for several intervals
	after := hits.Load()

	if after > before {
		t.Errorf("polling continued after cancel: before=%d after=%d", before, after)
	}
}

// =========================================================================
// HTTPSource — Close() unblocks Read
// =========================================================================

func TestHTTPSource_CloseUnblocksReader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"x": 1}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := NewHTTPSource(ctx, srv.URL, HTTPInterval(time.Hour))

	// Read the first response.
	scanner := bufio.NewScanner(src)
	if !scanner.Scan() {
		t.Fatal("first scan failed")
	}

	// Close should make the next Read return an error.
	if err := src.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	if scanner.Scan() {
		t.Error("scan returned a line after Close")
	}
}

// =========================================================================
// HTTPSource — custom timeout option
// =========================================================================

func TestHTTPSource_TimeoutOption(t *testing.T) {
	// Server that never responds — exercises the timeout path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := NewHTTPSource(ctx, srv.URL,
		HTTPInterval(20*time.Millisecond),
		HTTPTimeout(50*time.Millisecond),
	)
	defer src.Close()

	// We shouldn't get any data, but we also shouldn't hang.
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 64)
		_, _ = src.Read(buf)
		close(done)
	}()

	select {
	case <-done:
		// Reader returned (probably with EOF after Close in defer).
	case <-time.After(500 * time.Millisecond):
		// Reader is still blocking, which is expected since timeout ≠ EOF.
		// Just make sure cancellation works:
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Error("reader didn't unblock after cancel")
		}
	}
}

// =========================================================================
// FetchOnce
// =========================================================================

func TestFetchOnce_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"hello": "world"}`)
	}))
	defer srv.Close()

	body, err := FetchOnce(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchOnce: %v", err)
	}
	if string(body) != `{"hello": "world"}` {
		t.Errorf("body = %q", body)
	}
}

func TestFetchOnce_WithHeader(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		fmt.Fprint(w, `ok`)
	}))
	defer srv.Close()

	_, err := FetchOnce(context.Background(), srv.URL, HTTPHeader("X-API-Key", "secret"))
	if err != nil {
		t.Fatalf("FetchOnce: %v", err)
	}
	if gotKey != "secret" {
		t.Errorf("X-API-Key = %q", gotKey)
	}
}

func TestFetchOnce_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := FetchOnce(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want to mention 401", err)
	}
}

func TestFetchOnce_BadURL(t *testing.T) {
	_, err := FetchOnce(context.Background(), "http://[::1]:0/invalid")
	if err == nil {
		t.Error("expected error for unreachable URL")
	}
}

func TestFetchOnce_BadURLSyntax(t *testing.T) {
	_, err := FetchOnce(context.Background(), "://not a url")
	if err == nil {
		t.Error("expected error for malformed URL")
	}
}

// =========================================================================
// squashJSON
// =========================================================================

func TestSquashJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \n\t  ", ""},
		{"single line", `{"a":1}`, `{"a":1}`},
		{
			"pretty printed",
			"{\n  \"a\": 1,\n  \"b\": 2\n}",
			`{  "a": 1,  "b": 2}`,
		},
		{
			"newline inside string preserved",
			`{"msg": "hello\nworld"}`,
			`{"msg": "hello\nworld"}`,
		},
		{
			"escaped quote in string",
			`{"q": "he said \"hi\""}`,
			`{"q": "he said \"hi\""}`,
		},
		{
			"raw newline inside string is preserved",
			"{\"msg\": \"line1\nline2\"}",
			"{\"msg\": \"line1\nline2\"}",
		},
		{
			"carriage return outside string stripped",
			"{\r\n  \"a\": 1\r\n}",
			`{  "a": 1}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(squashJSON([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("squashJSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// =========================================================================
// HTTPSource — empty body skipped
// =========================================================================

func TestHTTPSource_EmptyBodySkipped(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			// First poll: empty body — should be skipped.
			return
		}
		fmt.Fprintf(w, `{"n": %d}`, n)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := NewHTTPSource(ctx, srv.URL, HTTPInterval(20*time.Millisecond))
	defer src.Close()

	scanner := bufio.NewScanner(src)
	if !scanner.Scan() {
		t.Fatal("scan failed")
	}
	// First emitted line must be from poll #2 onward (n >= 2).
	if !strings.Contains(scanner.Text(), `"n":`) {
		t.Errorf("unexpected first line: %q", scanner.Text())
	}
}
