package network

import (
	"net/url"
	"path"
	"strings"

	"github.com/example/GoLinkfinderEVO/internal/parser"
)

// CheckURL validates a JS endpoint and resolves it to an absolute URL based on the provided base.
func CheckURL(raw, base string) (string, bool) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", false
	}

	trimmed := candidate
	if idx := strings.IndexAny(trimmed, "?#"); idx != -1 {
		trimmed = trimmed[:idx]
	}

	lowerTrimmed := strings.ToLower(trimmed)
	if !parser.ScriptExtensionRegex().MatchString(lowerTrimmed) {
		return "", false
	}

	parts := strings.Split(lowerTrimmed, "/")
	for _, p := range parts {
		if p == "node_modules" || p == "jquery.js" {
			return "", false
		}
	}

	ref, err := url.Parse(candidate)
	if err != nil {
		return "", false
	}

	if ref.IsAbs() {
		return ref.String(), true
	}

	if strings.HasPrefix(candidate, "//") {
		resolved := "https:" + candidate
		return resolved, true
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return "", false
	}

	if baseURL.Scheme == "" {
		baseURL.Scheme = "https"
	}

	// Ensure the base URL represents a directory for relative resolution.
	if baseURL.Path != "" && !strings.HasSuffix(baseURL.Path, "/") {
		dir := path.Dir(baseURL.Path)
		if dir == "." {
			dir = "/"
		}
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		baseURL.Path = dir
	}

	resolved := baseURL.ResolveReference(ref)
	if resolved == nil || resolved.Scheme == "" {
		return "", false
	}

	return resolved.String(), true
}

// WithinScope reports whether the provided resource URL belongs to the supplied scope domain.
// The scope can be provided with or without a scheme (e.g. "https://example.com" or "example.com").
// When includeSubdomains is true, subdomains of the provided scope are also considered in-scope.
func WithinScope(resource, scope string, includeSubdomains bool) bool {
	if scope == "" {
		return true
	}

	parsedResource, err := url.Parse(resource)
	if err != nil {
		return false
	}

	resourceHost := parsedResource.Hostname()
	if resourceHost == "" {
		return false
	}

	parsedScope, err := url.Parse(scope)
	if err != nil || parsedScope.Hostname() == "" {
		parsedScope, err = url.Parse("https://" + scope)
		if err != nil {
			return false
		}
	}

	scopeHost := parsedScope.Hostname()
	if scopeHost == "" {
		return false
	}

	resourceHost = strings.ToLower(resourceHost)
	scopeHost = strings.ToLower(scopeHost)

	if !includeSubdomains {
		return resourceHost == scopeHost
	}

	if resourceHost == scopeHost {
		return true
	}

	return strings.HasSuffix(resourceHost, "."+scopeHost)
}
