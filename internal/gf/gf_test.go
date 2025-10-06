package gf

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/model"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/output"
)

func TestLoadDefinitionsFromDir(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "token.json")
	content := []byte(`{"pattern":"token","flags":"-i"}`)
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatalf("failed to write gf definition: %v", err)
	}

	defs, err := loadDefinitionsFromDir(dir, []string{"token"}, false)
	if err != nil {
		t.Fatalf("loadDefinitionsFromDir returned error: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}

	if defs[0].Name != "token" {
		t.Fatalf("expected rule name 'token', got %s", defs[0].Name)
	}

	if len(defs[0].Patterns) != 1 {
		t.Fatalf("expected 1 compiled pattern, got %d", len(defs[0].Patterns))
	}

	if !defs[0].Patterns[0].MatchString("ACCESS_TOKEN") {
		t.Fatalf("expected compiled pattern to honour case-insensitive flag")
	}

	defsAll, err := loadDefinitionsFromDir(dir, nil, true)
	if err != nil {
		t.Fatalf("loadDefinitionsFromDir (all) returned error: %v", err)
	}

	if len(defsAll) != 1 {
		t.Fatalf("expected 1 definition when loading all, got %d", len(defsAll))
	}
}

func TestFindInReports(t *testing.T) {
	defs := []Definition{{
		Name:     "token",
		Patterns: []*regexp.Regexp{regexp.MustCompile("token")},
	}}

	reports := []output.ResourceReport{{
		Resource: "app.js",
		Endpoints: []model.Endpoint{{
			Link: "/api/token/refresh",
			Line: 42,
		}},
	}}

	findings := FindInReports(reports, defs)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	got := findings[0]
	if got.Resource != "app.js" {
		t.Fatalf("expected resource 'app.js', got %s", got.Resource)
	}

	if got.Line != 42 {
		t.Fatalf("expected line 42, got %d", got.Line)
	}

	if got.Evidence != "token" {
		t.Fatalf("expected evidence 'token', got %s", got.Evidence)
	}

	if len(got.Rules) != 1 || got.Rules[0] != "token" {
		t.Fatalf("expected rules ['token'], got %v", got.Rules)
	}

	if got.Context != "" {
		t.Fatalf("expected empty context, got %q", got.Context)
	}
}

func TestFindInReportsMergesRulesAndContext(t *testing.T) {
	defs := []Definition{
		{
			Name:     "token",
			Patterns: []*regexp.Regexp{regexp.MustCompile("token")},
		},
		{
			Name:     "api",
			Patterns: []*regexp.Regexp{regexp.MustCompile("api")},
		},
	}

	reports := []output.ResourceReport{{
		Resource: "app.js",
		Endpoints: []model.Endpoint{{
			Link:    "/api/token/refresh",
			Context: "fetch('/api/token/refresh')",
			Line:    42,
		}},
	}}

	findings := FindInReports(reports, defs)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (token and api), got %d", len(findings))
	}

	for _, finding := range findings {
		if finding.Context == "" {
			t.Fatalf("expected finding context to be populated")
		}
		if len(finding.Rules) != 1 {
			t.Fatalf("expected each finding to have one rule, got %v", finding.Rules)
		}
	}

	// Ensure that matches for identical evidence aggregate rules together.
	defs = []Definition{
		{
			Name:     "token",
			Patterns: []*regexp.Regexp{regexp.MustCompile("token")},
		},
		{
			Name:     "token-alt",
			Patterns: []*regexp.Regexp{regexp.MustCompile("token")},
		},
	}

	findings = FindInReports(reports, defs)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when evidence is shared, got %d", len(findings))
	}

	got := findings[0]
	if len(got.Rules) != 2 {
		t.Fatalf("expected 2 aggregated rules, got %v", got.Rules)
	}

	if got.Context == "" {
		t.Fatalf("expected context to be kept when aggregating findings")
	}

	expectedEvidence := "token"
	if got.Evidence != expectedEvidence {
		t.Fatalf("expected evidence %q, got %q", expectedEvidence, got.Evidence)
	}
}
