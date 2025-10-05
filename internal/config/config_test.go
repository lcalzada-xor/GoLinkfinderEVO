package config

import (
	"flag"
	"os"
	"testing"
)

func TestParseFlagsJSON(t *testing.T) {
	t.Parallel()

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

	if cfg.JSON != "report.json" {
		t.Fatalf("expected JSON path to be %q, got %q", "report.json", cfg.JSON)
	}
}
