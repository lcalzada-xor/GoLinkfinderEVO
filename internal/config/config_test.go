package config

import (
	"flag"
	"os"
	"testing"
)

func TestParseFlagsJSON(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
	})

	flag.CommandLine = flag.NewFlagSet(oldArgs[0], flag.ContinueOnError)

	os.Args = []string{
		oldArgs[0],
		"-i", "https://example.com", "--json", "report.json",
	}

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags() returned error: %v", err)
	}

	jsonTarget, ok := findOutput(cfg.Outputs, OutputJSON)
	if !ok {
		t.Fatalf("expected JSON output to be configured")
	}

	if jsonTarget.Path != "report.json" {
		t.Fatalf("expected JSON path to be %q, got %q", "report.json", jsonTarget.Path)
	}

	// CLI is not added by default when other outputs are specified
	if _, ok := findOutput(cfg.Outputs, OutputCLI); ok {
		t.Fatalf("expected CLI output NOT to be configured when JSON is specified")
	}
}

func TestParseFlagsGFOutputs(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
	})

	flag.CommandLine = flag.NewFlagSet(oldArgs[0], flag.ContinueOnError)

	os.Args = []string{
		oldArgs[0],
		"--output", "cli,json",
		"-i", "https://example.com", "--gf", "all",
	}

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags() returned error: %v", err)
	}

	// Verify CLI output is configured
	_, ok := findOutput(cfg.Outputs, OutputCLI)
	if !ok {
		t.Fatalf("expected CLI output to be configured")
	}

	// Verify JSON output is configured
	jsonOut, ok := findOutput(cfg.Outputs, OutputJSON)
	if !ok {
		t.Fatalf("expected JSON output to be configured")
	}

	// JSON without path should be empty (stdout)
	if jsonOut.Path != "" {
		t.Fatalf("expected JSON path to be empty (stdout), got %q", jsonOut.Path)
	}

	// Verify GF is enabled
	if !cfg.GFAll {
		t.Fatalf("expected GF all to be enabled")
	}
}

func TestParseFlagsHeaders(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
	})

	flag.CommandLine = flag.NewFlagSet(oldArgs[0], flag.ContinueOnError)

	os.Args = []string{
		oldArgs[0],
		"-i", "https://example.com/resource.js",
		"--header", "Authorization: Bearer token",
		"-H", "User-Agent: Custom Agent",
		"--header", "x-test-header: some value",
	}

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags() returned error: %v", err)
	}

	if len(cfg.Headers) != 3 {
		t.Fatalf("expected 3 headers, got %d", len(cfg.Headers))
	}

	if got, want := cfg.Headers[0], (Header{Name: "Authorization", Value: "Bearer token"}); got != want {
		t.Fatalf("unexpected first header: %+v", got)
	}

	if got, want := cfg.Headers[1], (Header{Name: "User-Agent", Value: "Custom Agent"}); got != want {
		t.Fatalf("unexpected second header: %+v", got)
	}

	if got, want := cfg.Headers[2], (Header{Name: "X-Test-Header", Value: "some value"}); got != want {
		t.Fatalf("unexpected third header: %+v", got)
	}
}

func TestParseFlagsHeaderInvalidFormat(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
	})

	flag.CommandLine = flag.NewFlagSet(oldArgs[0], flag.ContinueOnError)

	os.Args = []string{
		oldArgs[0],
		"-i", "https://example.com",
		"--header", "missing colon",
	}

	if _, err := ParseFlags(); err == nil {
		t.Fatal("expected error due to invalid header format, got nil")
	}
}

func findOutput(outputs []OutputTarget, format OutputFormat) (OutputTarget, bool) {
	for _, target := range outputs {
		if target.Format == format {
			return target, true
		}
	}
	return OutputTarget{}, false
}
