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

	jsonTarget, ok := findOutput(cfg.Outputs, OutputJSON)
	if !ok {
		t.Fatalf("expected JSON output to be configured")
	}

	if jsonTarget.Path != "report.json" {
		t.Fatalf("expected JSON path to be %q, got %q", "report.json", jsonTarget.Path)
	}

	if _, ok := findOutput(cfg.Outputs, OutputCLI); !ok {
		t.Fatalf("expected CLI output to be configured by default")
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
