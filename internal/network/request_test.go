package network

import (
	"bytes"
	"compress/zlib"
	"net/http"
	"net/http/httptest"
	"sync"
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

			_, err := Fetch(server.URL, cfg)
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
