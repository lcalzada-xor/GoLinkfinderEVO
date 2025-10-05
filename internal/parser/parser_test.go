package parser

import (
	"regexp"
	"testing"
)

func TestFindEndpointsWithContextAndFilter(t *testing.T) {
	content := `fetch("https://example.com/static/app.js");\nconst script = "/assets/test.js";\nconst duplicate = "/assets/test.js";`

	regex := EndpointRegex()
	filter := regexp.MustCompile(`/assets/`)

	endpoints := FindEndpoints(content, regex, true, filter, true)

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	ep := endpoints[0]
	if ep.Link != "/assets/test.js" {
		t.Fatalf("unexpected endpoint link: %s", ep.Link)
	}

	if ep.Context == "" {
		t.Fatalf("expected context to be populated")
	}

	if !regexp.MustCompile(`script`).MatchString(ep.Context) {
		t.Fatalf("expected context to contain surrounding code, got %q", ep.Context)
	}

	if ep.Line != 2 {
		t.Fatalf("expected endpoint to be on line 2, got %d", ep.Line)
	}
}

func TestFindEndpointsWithoutContext(t *testing.T) {
	content := `var script = '/js/app.js';`
	regex := EndpointRegex()

	endpoints := FindEndpoints(content, regex, false, nil, false)

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	if endpoints[0].Context != "" {
		t.Fatalf("expected context to be empty when includeContext is false")
	}

	if endpoints[0].Line != 1 {
		t.Fatalf("expected endpoint to be on line 1, got %d", endpoints[0].Line)
	}
}

func TestFindEndpointsWithBacktickDelimiters(t *testing.T) {
	content := "const users = fetch(`/api/users`);\nconst posts = fetch('/api/posts');"
	regex := EndpointRegex()

	endpoints := FindEndpoints(content, regex, false, nil, false)

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	links := map[string]struct{}{}
	for _, ep := range endpoints {
		links[ep.Link] = struct{}{}
	}

	if _, ok := links["/api/users"]; !ok {
		t.Fatalf("expected to find /api/users endpoint in results: %#v", links)
	}
	if _, ok := links["/api/posts"]; !ok {
		t.Fatalf("expected to find /api/posts endpoint in results: %#v", links)
	}
}

func TestHighlightContext(t *testing.T) {
	context := `<script src="/js/app.js?version=1"></script>`
	link := `/js/app.js?version=1`

	highlighted := HighlightContext(context, link)

	if highlighted == context {
		t.Fatalf("expected context to be highlighted")
	}

	if !regexp.MustCompile(`<mark class='highlight'>`).MatchString(highlighted) {
		t.Fatalf("expected highlighted mark element in context, got %q", highlighted)
	}

	if !regexp.MustCompile(regexp.QuoteMeta(link)).MatchString(highlighted) {
		t.Fatalf("highlighted context should contain the link text, got %q", highlighted)
	}
}

func TestEndpointRegexMatchesHostnamesWithoutDots(t *testing.T) {
	content := `'http://localhost:3000/api' "http://my-service/internal"`

	endpoints := FindEndpoints(content, EndpointRegex(), false, nil, false)

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	expected := []string{"http://localhost:3000/api", "http://my-service/internal"}
	for i, ep := range endpoints {
		if ep.Link != expected[i] {
			t.Fatalf("unexpected endpoint at index %d: got %q, want %q", i, ep.Link, expected[i])
		}
	}
}

func TestEndpointRegexMatchesExtendedExtensions(t *testing.T) {
	content := `"/api/handler.php5" '/assets/config.json5' "/configuration/app.config" "../services/process.ashx"`

	endpoints := FindEndpoints(content, EndpointRegex(), false, nil, false)

	expected := map[string]struct{}{
		"/api/handler.php5":         {},
		"/assets/config.json5":      {},
		"/configuration/app.config": {},
		"../services/process.ashx":  {},
	}

	for _, ep := range endpoints {
		delete(expected, ep.Link)
	}

	if len(expected) != 0 {
		t.Fatalf("expected endpoints for extended extensions to be matched, missing: %#v", expected)
	}
}
