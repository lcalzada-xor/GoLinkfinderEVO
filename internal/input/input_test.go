package input

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/config"
)

func TestResolveTargetsHTTP(t *testing.T) {
	cfg := config.Config{Input: "https://example.com/app.js"}
	targets, err := ResolveTargets(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].URL != cfg.Input {
		t.Fatalf("expected target URL %q, got %q", cfg.Input, targets[0].URL)
	}
	if targets[0].Prefetched {
		t.Fatalf("http targets should not be marked as prefetched")
	}
}

func TestResolveTargetsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "script.js")
	if err := os.WriteFile(file, []byte("console.log('test');"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg := config.Config{Input: file}
	targets, err := ResolveTargets(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	wantPrefix := "file://"
	if !strings.HasPrefix(targets[0].URL, wantPrefix) {
		t.Fatalf("expected file target to have %q prefix, got %q", wantPrefix, targets[0].URL)
	}
}

func TestResolveTargetsListFile(t *testing.T) {
	dir := t.TempDir()

	localFile := filepath.Join(dir, "local.js")
	if err := os.WriteFile(localFile, []byte("console.log('local');"), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	listPath := filepath.Join(dir, "targets.txt")
	content := strings.Join([]string{
		"https://example.com/app.js",
		"# comment",
		filepath.Base(localFile),
		"",
		"view-source:https://example.com/another.js",
	}, "\n")
	if err := os.WriteFile(listPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write list file: %v", err)
	}

	cfg := config.Config{Input: listPath}
	targets, err := ResolveTargets(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}

	wantFirst := "https://example.com/app.js"
	if targets[0].URL != wantFirst {
		t.Fatalf("expected first target %q, got %q", wantFirst, targets[0].URL)
	}

	wantThird := "https://example.com/another.js"
	if targets[2].URL != wantThird {
		t.Fatalf("expected third target %q, got %q", wantThird, targets[2].URL)
	}

	if !strings.HasPrefix(targets[1].URL, "file://") {
		t.Fatalf("expected second target to reference local file, got %q", targets[1].URL)
	}
}

func TestResolveTargetsGlob(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.js", "b.js", "ignore.txt"}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("console.log('test');"), 0o644); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
	}

	pattern := filepath.Join(dir, "*.js")
	cfg := config.Config{Input: pattern}
	targets, err := ResolveTargets(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
}

func TestResolveTargetsBurp(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "burp.xml")

	body := base64.StdEncoding.EncodeToString([]byte("console.log('test');"))
	data := `<items><item><url>https://example.com/app.js</url><response>` + body + `</response></item></items>`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write burp file: %v", err)
	}

	cfg := config.Config{Input: file, Burp: true}
	targets, err := ResolveTargets(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if !targets[0].Prefetched {
		t.Fatalf("burp targets should be prefetched")
	}
	if targets[0].Content == "" {
		t.Fatalf("expected prefetched content to be populated")
	}
}

func TestResolveFilePath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "folder with space", "script.js")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	contents := "console.log('test');"
	if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	abs, err := filepath.Abs(file)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	url := "file://" + filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		url = "file:///" + filepath.ToSlash(abs)
	}

	data, err := ResolveFilePath(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != contents {
		t.Fatalf("expected file contents %q, got %q", contents, data)
	}
}
