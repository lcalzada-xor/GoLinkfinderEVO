package network

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/config"
)

type clientSettings struct {
	proxy    string
	insecure bool
	timeout  time.Duration
}

var (
	clientMu      sync.Mutex
	sharedClient  *http.Client
	sharedSetting clientSettings
)

func getHTTPClient(cfg config.Config) (*http.Client, error) {
	desired := clientSettings{
		proxy:    cfg.Proxy,
		insecure: cfg.Insecure,
		timeout:  cfg.Timeout,
	}

	clientMu.Lock()
	defer clientMu.Unlock()

	if sharedClient != nil && sharedSetting == desired {
		return sharedClient, nil
	}

	transport, err := buildTransport(cfg)
	if err != nil {
		return nil, err
	}

	sharedClient = &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}
	sharedSetting = desired

	return sharedClient, nil
}

func buildTransport(cfg config.Config) (*http.Transport, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("unexpected default transport type")
	}

	transport := base.Clone()

	if cfg.Timeout > 0 {
		transport.TLSHandshakeTimeout = cfg.Timeout
		transport.ResponseHeaderTimeout = cfg.Timeout
	}

	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	if cfg.Insecure {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		} else {
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	return transport, nil
}

// Fetch retrieves the content for the provided URL.
func Fetch(rawURL string, cfg config.Config) (string, error) {
	client, err := getHTTPClient(cfg)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	if cfg.Cookies != "" {
		req.Header.Set("Cookie", cfg.Cookies)
	}

	for _, header := range cfg.Headers {
		if header.Name == "" {
			continue
		}
		req.Header.Set(header.Name, header.Value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := readResponseBody(resp)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// resetHTTPClient clears the shared HTTP client. It is intended for use in tests.
func resetHTTPClient() {
	clientMu.Lock()
	defer clientMu.Unlock()
	sharedClient = nil
	sharedSetting = clientSettings{}
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if encoding == "" {
		return raw, nil
	}

	// Some servers return multiple encodings separated by commas. Only the first
	// value is considered as the effective encoding for this response.
	if idx := strings.IndexByte(encoding, ','); idx != -1 {
		encoding = strings.TrimSpace(encoding[:idx])
	}

	var (
		decoded []byte
		decErr  error
	)

	switch encoding {
	case "gzip":
		decoded, decErr = decompressBytes(raw, func(r io.Reader) (io.ReadCloser, error) {
			return gzip.NewReader(r)
		})
	case "deflate":
		decoded, decErr = decompressBytes(raw, func(r io.Reader) (io.ReadCloser, error) {
			return zlib.NewReader(r)
		})
	case "br":
		decoded, decErr = decompressBytes(raw, func(r io.Reader) (io.ReadCloser, error) {
			return io.NopCloser(brotli.NewReader(r)), nil
		})
	default:
		return raw, nil
	}

	if decErr == nil {
		return decoded, nil
	}

	if len(decoded) > 0 && isRecoverableDecompressionError(decErr) {
		return decoded, nil
	}
	if isRecoverableDecompressionError(decErr) {
		return raw, nil
	}

	return nil, decErr
}

func decompressBytes(data []byte, opener func(io.Reader) (io.ReadCloser, error)) ([]byte, error) {
	reader, err := opener(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decoded, readErr := io.ReadAll(reader)
	if readErr != nil {
		return decoded, readErr
	}

	return decoded, nil
}

func isRecoverableDecompressionError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, gzip.ErrHeader) ||
		errors.Is(err, gzip.ErrChecksum) ||
		errors.Is(err, zlib.ErrChecksum) ||
		errors.Is(err, zlib.ErrHeader)
}

// IsTimeoutError reports whether err represents a timeout condition.
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if os.IsTimeout(err) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return IsTimeoutError(urlErr.Err)
	}

	type timeout interface {
		Timeout() bool
	}

	var t timeout
	if errors.As(err, &t) && t.Timeout() {
		return true
	}

	msg := err.Error()
	if strings.Contains(msg, "Client.Timeout exceeded") || strings.Contains(msg, "context deadline exceeded") {
		return true
	}

	return false
}

// IsDNSOrNetworkError reports whether err represents a DNS resolution or network error
// that can be safely ignored (e.g., host doesn't exist, connection refused).
func IsDNSOrNetworkError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	// Check for common DNS and network errors
	return strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "host is down") ||
		strings.Contains(msg, "dial tcp") && strings.Contains(msg, "lookup") ||
		strings.Contains(msg, "name resolution failed") ||
		strings.Contains(msg, "temporary failure in name resolution")
}
