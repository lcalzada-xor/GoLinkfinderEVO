package parser

import (
	"regexp"
	"testing"
	"time"

	"github.com/ditashi/jsbeautifier-go/optargs"
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

func TestEndpointRegexSupportsExtendedSchemes(t *testing.T) {
	content := `'chrome-extension://example/extensions.js' "//cdn.example.com/lib.js"`

	endpoints := FindEndpoints(content, EndpointRegex(), false, nil, false)

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	expected := []string{"chrome-extension://example/extensions.js", "//cdn.example.com/lib.js"}
	for i, ep := range endpoints {
		if ep.Link != expected[i] {
			t.Fatalf("unexpected endpoint at index %d: got %q, want %q", i, ep.Link, expected[i])
		}
	}
}

func TestEndpointRegexSupportsExtendedFileExtensions(t *testing.T) {
	content := `"/api/status.php5" '/config/app.config' 
"/static/app.json5" '/handler/process.ashx'`

	endpoints := FindEndpoints(content, EndpointRegex(), false, nil, false)

	if len(endpoints) != 4 {
		t.Fatalf("expected 4 endpoints, got %d", len(endpoints))
	}

	expected := map[string]struct{}{
		"/api/status.php5":      {},
		"/config/app.config":    {},
		"/static/app.json5":     {},
		"/handler/process.ashx": {},
	}

	for _, ep := range endpoints {
		if _, ok := expected[ep.Link]; !ok {
			t.Fatalf("unexpected endpoint found: %q", ep.Link)
		}
		delete(expected, ep.Link)
	}

	if len(expected) != 0 {
		t.Fatalf("expected endpoints were not all matched: %#v", expected)
	}
}

func TestBeautifyTimeoutFallback(t *testing.T) {
	originalFunc := beautifyFunc
	originalTimeout := beautifyTimeout
	t.Cleanup(func() {
		beautifyFunc = originalFunc
		beautifyTimeout = originalTimeout
	})

	beautifyTimeout = 10 * time.Millisecond
	delay := beautifyTimeout * 10

	beautifyFunc = func(src *string, _ optargs.MapType) (string, error) {
		time.Sleep(delay)
		return "beautified", nil
	}

	input := "const value = fetch('/api');"
	start := time.Now()
	result := beautify(input)
	elapsed := time.Since(start)

	if result != input {
		t.Fatalf("expected original content when beautifier times out, got %q", result)
	}

	if elapsed > beautifyTimeout*5 {
		t.Fatalf("beautify took too long to return after timeout: %v", elapsed)
	}
}
