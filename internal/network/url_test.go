package network

import "testing"

func TestCheckURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		base    string
		wantURL string
		wantOK  bool
	}{
		{
			name:    "absolute url",
			raw:     "https://cdn.example.com/app.js",
			base:    "https://example.com/index.html",
			wantURL: "https://cdn.example.com/app.js",
			wantOK:  true,
		},
		{
			name:    "protocol relative",
			raw:     "//cdn.example.com/app.js",
			base:    "https://example.com/index.html",
			wantURL: "https://cdn.example.com/app.js",
			wantOK:  true,
		},
		{
			name:    "relative path",
			raw:     "scripts/app.js",
			base:    "https://example.com/app/page.html",
			wantURL: "https://example.com/app/scripts/app.js",
			wantOK:  true,
		},
		{
			name:   "ignore node_modules",
			raw:    "/node_modules/jquery.js",
			base:   "https://example.com/app/page.html",
			wantOK: false,
		},
		{
			name:   "require js extension",
			raw:    "/styles/app.css",
			base:   "https://example.com/app/page.html",
			wantOK: false,
		},
		{
			name:    "relative with query",
			raw:     "scripts/app.js?v=1",
			base:    "https://example.com/app/page.html",
			wantURL: "https://example.com/app/scripts/app.js?v=1",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CheckURL(tt.raw, tt.base)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if !tt.wantOK {
				return
			}
			if got != tt.wantURL {
				t.Fatalf("expected url %q, got %q", tt.wantURL, got)
			}
		})
	}
}

func TestWithinScope(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		scope    string
		include  bool
		want     bool
	}{
		{
			name:     "matching host with scheme",
			resource: "https://static.example.com/app.js",
			scope:    "https://static.example.com",
			include:  false,
			want:     true,
		},
		{
			name:     "matching host without scope scheme",
			resource: "https://example.com/app.js",
			scope:    "example.com",
			include:  false,
			want:     true,
		},
		{
			name:     "different host",
			resource: "https://cdn.example.com/app.js",
			scope:    "example.com",
			include:  false,
			want:     false,
		},
		{
			name:     "ignore port in resource",
			resource: "https://example.com:8443/app.js",
			scope:    "https://example.com",
			include:  false,
			want:     true,
		},
		{
			name:     "invalid scope",
			resource: "https://example.com/app.js",
			scope:    "://",
			include:  false,
			want:     false,
		},
		{
			name:     "include subdomain when enabled",
			resource: "https://cdn.example.com/app.js",
			scope:    "example.com",
			include:  true,
			want:     true,
		},
		{
			name:     "include subdomain when scope has scheme",
			resource: "https://cdn.example.com/app.js",
			scope:    "https://example.com/",
			include:  true,
			want:     true,
		},
		{
			name:     "include nested subdomain when enabled",
			resource: "https://a.b.example.com/app.js",
			scope:    "example.com",
			include:  true,
			want:     true,
		},
		{
			name:     "subdomain disabled remains out of scope",
			resource: "https://cdn.example.com/app.js",
			scope:    "example.com",
			include:  false,
			want:     false,
		},
		{
			name:     "subdomain disabled when scope has scheme",
			resource: "https://cdn.example.com/app.js",
			scope:    "https://example.com/",
			include:  false,
			want:     false,
		},
		{
			name:     "suffix that is not a subdomain",
			resource: "https://badexample.com/app.js",
			scope:    "example.com",
			include:  true,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WithinScope(tt.resource, tt.scope, tt.include)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
