package network

import (
	"net/url"
	"path"
	"strings"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/parser"
)

// ResourceType represents the type of resource discovered
type ResourceType int

const (
	// ResourceUnknown represents an unrecognized resource type
	ResourceUnknown ResourceType = iota
	// ResourceJavaScript represents a JavaScript file
	ResourceJavaScript
	// ResourceSitemap represents an XML sitemap
	ResourceSitemap
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

// DetectResourceType determines the type of resource based on the URL.
func DetectResourceType(raw string) ResourceType {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return ResourceUnknown
	}

	// Remove query parameters and fragments for extension checking
	trimmed := candidate
	if idx := strings.IndexAny(trimmed, "?#"); idx != -1 {
		trimmed = trimmed[:idx]
	}

	lowerTrimmed := strings.ToLower(trimmed)

	// Check for sitemap indicators
	if strings.HasSuffix(lowerTrimmed, ".xml") || strings.Contains(lowerTrimmed, "sitemap") {
		return ResourceSitemap
	}

	// Check for JavaScript extensions
	if parser.ScriptExtensionRegex().MatchString(lowerTrimmed) {
		// Filter out common dependencies
		parts := strings.Split(lowerTrimmed, "/")
		for _, p := range parts {
			if p == "node_modules" || p == "jquery.js" {
				return ResourceUnknown
			}
		}
		return ResourceJavaScript
	}

	return ResourceUnknown
}

// ResolveURL validates and resolves a resource URL based on its type and the provided base.
// It returns the resolved URL, the resource type, and a boolean indicating success.
func ResolveURL(raw, base string, allowedTypes ...ResourceType) (string, ResourceType, bool) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", ResourceUnknown, false
	}

	resourceType := DetectResourceType(candidate)
	if resourceType == ResourceUnknown {
		return "", ResourceUnknown, false
	}

	// Check if this resource type is allowed
	if len(allowedTypes) > 0 {
		allowed := false
		for _, allowedType := range allowedTypes {
			if resourceType == allowedType {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", resourceType, false
		}
	}

	ref, err := url.Parse(candidate)
	if err != nil {
		return "", resourceType, false
	}

	if ref.IsAbs() {
		return ref.String(), resourceType, true
	}

	if strings.HasPrefix(candidate, "//") {
		resolved := "https:" + candidate
		return resolved, resourceType, true
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return "", resourceType, false
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
		return "", resourceType, false
	}

	return resolved.String(), resourceType, true
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
