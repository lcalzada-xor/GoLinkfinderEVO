package input

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/example/GoLinkfinderEVO/internal/config"
	"github.com/example/GoLinkfinderEVO/internal/model"
)

// ResolveTargets returns the list of targets to evaluate based on the provided configuration.
func ResolveTargets(cfg config.Config) ([]model.Target, error) {
	input := cfg.Input

	if strings.HasPrefix(input, "view-source:") {
		input = input[12:]
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") ||
		strings.HasPrefix(input, "file://") || strings.HasPrefix(input, "ftp://") ||
		strings.HasPrefix(input, "ftps://") {
		if cfg.Burp {
			return nil, errors.New("burp mode requires a file input")
		}
		return []model.Target{{URL: input}}, nil
	}

	if cfg.Burp {
		return parseBurpFile(input)
	}

	if strings.Contains(input, "*") {
		return resolveGlob(input)
	}

	if _, err := os.Stat(input); err == nil {
		abs, err := filepath.Abs(input)
		if err != nil {
			return nil, err
		}
		return []model.Target{{URL: "file://" + abs}}, nil
	}

	return nil, errors.New("file could not be found (maybe you forgot to add http/https)")
}

type burpItem struct {
	URL      string `xml:"url"`
	Response struct {
		Text string `xml:",chardata"`
	} `xml:"response"`
}

type burpDocument struct {
	Items []burpItem `xml:"item"`
}

func parseBurpFile(path string) ([]model.Target, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc burpDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	var targets []model.Target
	for _, item := range doc.Items {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(item.Response.Text))
		if err != nil {
			return nil, err
		}
		targets = append(targets, model.Target{URL: item.URL, Content: string(decoded), Prefetched: true})
	}

	return targets, nil
}

func resolveGlob(pattern string) ([]model.Target, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, errors.New("input with wildcard does not match any files")
	}

	var targets []model.Target
	for _, path := range matches {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			abs, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			targets = append(targets, model.Target{URL: "file://" + abs})
		}
	}

	if len(targets) == 0 {
		return nil, errors.New("input with wildcard does not match any files")
	}

	return targets, nil
}

// ResolveFilePath resolves a file:// URL to an absolute path and returns its contents.
func ResolveFilePath(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	path, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", err
	}
	if path == "" {
		path = strings.TrimPrefix(rawURL, "file://")
	}
	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	path = filepath.FromSlash(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
