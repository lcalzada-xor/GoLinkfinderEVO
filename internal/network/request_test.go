package network

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/config"
)

func TestFetchDeflate(t *testing.T) {
	resetHTTPClient()
	t.Cleanup(resetHTTPClient)

	const payload = "compressed content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		if _, err := zw.Write([]byte(payload)); err != nil {
			t.Fatalf("failed to write deflate payload: %v", err)
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("failed to close deflate writer: %v", err)
		}

		w.Header().Set("Content-Encoding", "deflate")
		if _, err := w.Write(buf.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.Config{Timeout: time.Second}
	content, err := Fetch(context.Background(), server.URL, cfg)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if content != payload {
		t.Fatalf("unexpected content: got %q want %q", content, payload)
	}
}

func TestFetchBrotli(t *testing.T) {
	resetHTTPClient()
	t.Cleanup(resetHTTPClient)

	const payload = "brotli compressed content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		bw := brotli.NewWriter(&buf)
		if _, err := bw.Write([]byte(payload)); err != nil {
			t.Fatalf("failed to write brotli payload: %v", err)
		}
		if err := bw.Close(); err != nil {
			t.Fatalf("failed to close brotli writer: %v", err)
		}

		w.Header().Set("Content-Encoding", "br")
		if _, err := w.Write(buf.Bytes()); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.Config{Timeout: time.Second}
	content, err := Fetch(context.Background(), server.URL, cfg)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if content != payload {
		t.Fatalf("unexpected content: got %q want %q", content, payload)
	}
}

func TestFetchAppliesCustomHeaders(t *testing.T) {
	resetHTTPClient()
	t.Cleanup(resetHTTPClient)

	const (
		token     = "Bearer custom-token"
		userAgent = "GoLinkFinderEVO/headers-test"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != token {
			t.Fatalf("unexpected Authorization header: got %q want %q", got, token)
		}
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Fatalf("unexpected User-Agent header: got %q want %q", got, userAgent)
		}
		if got := r.Header.Get("X-Test-Header"); got != "value" {
			t.Fatalf("unexpected X-Test-Header: got %q want %q", got, "value")
		}
		if got := r.Header.Get("Accept-Language"); got == "" {
			t.Fatal("expected default Accept-Language header to be set")
		}

		if _, err := w.Write([]byte("ok")); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.Config{
		Timeout: time.Second,
		Headers: []config.Header{
			{Name: "Authorization", Value: token},
			{Name: "User-Agent", Value: userAgent},
			{Name: "X-Test-Header", Value: "value"},
		},
	}

	if _, err := Fetch(server.URL, cfg); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
}

func TestFetchHandlesCorruptGzip(t *testing.T) {
	resetHTTPClient()
	t.Cleanup(resetHTTPClient)

	const payload = "this is some gzipped content that will be truncated"

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(payload)); err != nil {
		t.Fatalf("failed to write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	truncated := buf.Bytes()[:len(buf.Bytes())/2]

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		if _, err := w.Write(truncated); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.Config{Timeout: time.Second}
	content, err := Fetch(context.Background(), server.URL, cfg)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if content == "" {
		t.Fatal("expected partial content but received empty string")
	}

	if !strings.HasPrefix(payload, content) && !strings.HasPrefix(content, payload) {
		t.Fatalf("unexpected decompressed content: %q", content)
	}
}

func TestFetchConcurrentRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.Config{Timeout: time.Second}

	const workers = 8
	var active int32
	var maxActive int32

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			current := atomic.AddInt32(&active, 1)
			for {
				prev := atomic.LoadInt32(&maxActive)
				if current <= prev {
					break
				}
				if atomic.CompareAndSwapInt32(&maxActive, prev, current) {
					break
				}
			}

			_, err := Fetch(context.Background(), server.URL, cfg)
			errs <- err

			atomic.AddInt32(&active, -1)
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Fetch returned error: %v", err)
		}
	}

	if maxActive < 2 {
		t.Fatalf("expected concurrent requests, max active = %d", maxActive)
	}
}

func TestBuildTransportRespectsTimeout(t *testing.T) {
	cfg := config.Config{Timeout: 30 * time.Second}

	transport, err := buildTransport(cfg)
	if err != nil {
		t.Fatalf("buildTransport returned error: %v", err)
	}

	if got, want := transport.TLSHandshakeTimeout, cfg.Timeout; got != want {
		t.Fatalf("TLSHandshakeTimeout = %v, want %v", got, want)
	}

	if got, want := transport.ResponseHeaderTimeout, cfg.Timeout; got != want {
		t.Fatalf("ResponseHeaderTimeout = %v, want %v", got, want)
	}
}

func TestBuildTransportZeroTimeout(t *testing.T) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Skip("default transport is not *http.Transport")
	}

	cfg := config.Config{Timeout: 0}

	transport, err := buildTransport(cfg)
	if err != nil {
		t.Fatalf("buildTransport returned error: %v", err)
	}

	if transport.TLSHandshakeTimeout != base.TLSHandshakeTimeout {
		t.Fatalf("TLSHandshakeTimeout = %v, want %v", transport.TLSHandshakeTimeout, base.TLSHandshakeTimeout)
	}

	if transport.ResponseHeaderTimeout != base.ResponseHeaderTimeout {
		t.Fatalf("ResponseHeaderTimeout = %v, want %v", transport.ResponseHeaderTimeout, base.ResponseHeaderTimeout)
	}
}

func TestIsTimeoutError(t *testing.T) {
	t.Parallel()

	errTimeout := mockTimeoutError{}
	errURL := &url.Error{Err: context.DeadlineExceeded}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context deadline", err: context.DeadlineExceeded, want: true},
		{name: "os timeout", err: os.ErrDeadlineExceeded, want: true},
		{name: "net timeout", err: errTimeout, want: true},
		{name: "url error timeout", err: errURL, want: true},
		{name: "client timeout string", err: errors.New("Get \"https://example.com\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"), want: true},
		{name: "non timeout", err: errors.New("boom"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTimeoutError(tc.err); got != tc.want {
				t.Fatalf("IsTimeoutError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

type mockTimeoutError struct{}

func (mockTimeoutError) Error() string   { return "timeout" }
func (mockTimeoutError) Timeout() bool   { return true }
func (mockTimeoutError) Temporary() bool { return false }
