package network

import (
	"bytes"
	"compress/zlib"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/GoLinkfinderEVO/internal/config"
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
	content, err := Fetch(server.URL, cfg)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if content != payload {
		t.Fatalf("unexpected content: got %q want %q", content, payload)
	}
}

func TestFetchWithProxy(t *testing.T) {
	resetHTTPClient()
	t.Cleanup(resetHTTPClient)

	const payload = "proxied response"
	const targetURL = "http://example.com/resource"

	var proxied atomic.Bool

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Store(true)

		if got := r.URL.String(); got != targetURL {
			t.Fatalf("unexpected upstream URL: got %q want %q", got, targetURL)
		}

		if _, err := w.Write([]byte(payload)); err != nil {
			t.Fatalf("failed to write proxy response: %v", err)
		}
	}))
	defer proxy.Close()

	cfg := config.Config{Timeout: time.Second, Proxy: proxy.URL}
	content, err := Fetch(targetURL, cfg)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if content != payload {
		t.Fatalf("unexpected content: got %q want %q", content, payload)
	}

	if !proxied.Load() {
		t.Fatalf("expected request to be routed through the proxy")
	}
}

func TestFetchInsecureTLS(t *testing.T) {
	resetHTTPClient()
	t.Cleanup(resetHTTPClient)

	const payload = "secure content"

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(payload)); err != nil {
			t.Fatalf("failed to write TLS response: %v", err)
		}
	}))
	defer server.Close()

	strictCfg := config.Config{Timeout: time.Second}
	if _, err := Fetch(server.URL, strictCfg); err == nil {
		t.Fatalf("expected TLS verification error without --insecure")
	}

	insecureCfg := config.Config{Timeout: time.Second, Insecure: true}
	content, err := Fetch(server.URL, insecureCfg)
	if err != nil {
		t.Fatalf("Fetch returned error with --insecure: %v", err)
	}

	if content != payload {
		t.Fatalf("unexpected content: got %q want %q", content, payload)
	}
}
