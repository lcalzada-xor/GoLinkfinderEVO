package network

import (
	"bytes"
	"compress/zlib"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/GoLinkfinderEVO/internal/config"
)

func TestFetchDeflate(t *testing.T) {
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
