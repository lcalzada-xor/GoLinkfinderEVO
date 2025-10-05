package network

import (
	"compress/gzip"
	"compress/zlib"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/example/GoLinkfinderEVO/internal/config"
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
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	if cfg.Cookies != "" {
		req.Header.Set("Cookie", cfg.Cookies)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	reader, err := decodeBody(resp)
	if err != nil {
		return "", err
	}
	if reader != resp.Body {
		defer reader.Close()
	}

	data, err := io.ReadAll(reader)
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

func decodeBody(resp *http.Response) (io.ReadCloser, error) {
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		return gz, nil
	case "deflate":
		zr, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		return zr, nil
	default:
		return resp.Body, nil
	}
}
