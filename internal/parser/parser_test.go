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

func TestFindEndpointsWithExtendedExtensions(t *testing.T) {
	content := `const urls = ["/api/user.php5", "/static/data.json5?cache=1", "config.ashx", "settings.config"];`
	regex := EndpointRegex()

	endpoints := FindEndpoints(content, regex, false, nil, false)

	expected := map[string]bool{
		"/api/user.php5":             false,
		"/static/data.json5?cache=1": false,
		"config.ashx":                false,
		"settings.config":            false,
	}

	if len(endpoints) != len(expected) {
		t.Fatalf("expected %d endpoints, got %d", len(expected), len(endpoints))
	}

	for _, ep := range endpoints {
		seen, ok := expected[ep.Link]
		if !ok {
			t.Fatalf("unexpected endpoint %q", ep.Link)
		}
		if seen {
			t.Fatalf("endpoint %q reported multiple times", ep.Link)
		}
		expected[ep.Link] = true
	}

	for link, seen := range expected {
		if !seen {
			t.Fatalf("expected to find endpoint %q", link)
		}
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
