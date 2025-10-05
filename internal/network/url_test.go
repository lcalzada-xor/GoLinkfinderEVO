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
		want     bool
	}{
		{
			name:     "matching host with scheme",
			resource: "https://static.example.com/app.js",
			scope:    "https://static.example.com",
			want:     true,
		},
		{
			name:     "matching host without scope scheme",
			resource: "https://example.com/app.js",
			scope:    "example.com",
			want:     true,
		},
		{
			name:     "different host",
			resource: "https://cdn.example.com/app.js",
			scope:    "example.com",
			want:     false,
		},
		{
			name:     "ignore port in resource",
			resource: "https://example.com:8443/app.js",
			scope:    "https://example.com",
			want:     true,
		},
		{
			name:     "invalid scope",
			resource: "https://example.com/app.js",
			scope:    "://",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WithinScope(tt.resource, tt.scope)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
