package network

import (
	"testing"

	"github.com/example/GoLinkfinderEVO/internal/parser"
)

func TestCheckURLScriptExtensions(t *testing.T) {
	defaultExts := []string{".js", ".mjs", ".jsx", ".ts", ".tsx"}
	parser.SetScriptExtensions(defaultExts)
	t.Cleanup(func() {
		parser.SetScriptExtensions(defaultExts)
	})

	base := "https://example.com/index.html"

	tests := []struct {
		name string
		raw  string
		ok   bool
		want string
	}{
		{
			name: "JavaScript",
			raw:  "app.js",
			ok:   true,
			want: "https://example.com/app.js",
		},
		{
			name: "ECMAScript module",
			raw:  "https://cdn.example.com/app.mjs",
			ok:   true,
			want: "https://cdn.example.com/app.mjs",
		},
		{
			name: "JSX with query and fragment",
			raw:  "/assets/app.jsx?v=1#section",
			ok:   true,
			want: "https://example.com/assets/app.jsx?v=1#section",
		},
		{
			name: "TypeScript",
			raw:  "//static.example.com/app.ts",
			ok:   true,
			want: "https://static.example.com/app.ts",
		},
		{
			name: "TSX uppercase",
			raw:  "SCRIPTS/MAIN.TSX",
			ok:   true,
			want: "https://example.com/SCRIPTS/MAIN.TSX",
		},
		{
			name: "Non script extension",
			raw:  "styles.css",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CheckURL(tt.raw, base)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v (result %q)", tt.ok, ok, got)
			}
			if ok && got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
