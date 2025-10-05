package network

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/example/GoLinkfinderEVO/internal/config"
)

// Fetch retrieves the content for the provided URL.
func Fetch(rawURL string, cfg config.Config) (string, error) {
	client := &http.Client{Timeout: cfg.Timeout}
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

func decodeBody(resp *http.Response) (io.ReadCloser, error) {
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		return gz, nil
	case "deflate":
		fl := flate.NewReader(resp.Body)
		return fl, nil
	default:
		return resp.Body, nil
	}
}
