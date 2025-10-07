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

	if _, ok := findOutput(cfg.Outputs, OutputCLI); !ok {
		t.Fatalf("expected CLI output to be configured by default")
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
		"--output", "cli,gf.txt=findings.txt,gf.json=findings.json",
		"-i", "https://example.com", "--gf", "all",
	}

	cfg, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags() returned error: %v", err)
	}

	gfText, ok := findOutput(cfg.Outputs, OutputGFText)
	if !ok {
		t.Fatalf("expected gf text output to be configured")
	}

	if gfText.Path != "findings.txt" {
		t.Fatalf("expected gf text path to be %q, got %q", "findings.txt", gfText.Path)
	}

	gfJSON, ok := findOutput(cfg.Outputs, OutputGFJSON)
	if !ok {
		t.Fatalf("expected gf JSON output to be configured")
	}

	if gfJSON.Path != "findings.json" {
		t.Fatalf("expected gf JSON path to be %q, got %q", "findings.json", gfJSON.Path)
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
