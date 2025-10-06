package gf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// TextFilename is the default filename for plaintext gf findings.
	TextFilename = "gf.txt"
	// JSONFilename is the default filename for JSON gf findings.
	JSONFilename = "gf.json"
)

// WriteText creates a plaintext summary of gf findings.
func WriteText(path string, generatedAt time.Time, rules []string, findings []Finding) error {
	var buf bytes.Buffer

	buf.WriteString("# GoLinkfinderEVO gf findings\n")
	buf.WriteString(fmt.Sprintf("# Generated at: %s\n", generatedAt.Format(time.RFC3339)))
	if len(rules) > 0 {
		buf.WriteString(fmt.Sprintf("# Rules: %s\n", strings.Join(rules, ", ")))
	} else {
		buf.WriteString("# Rules: none\n")
	}
	buf.WriteString(fmt.Sprintf("# Total findings: %d\n\n", len(findings)))

	if len(findings) == 0 {
		buf.WriteString("# No gf findings were detected.\n")
	} else {
		for _, finding := range findings {
			buf.WriteString("[Resource] ")
			buf.WriteString(finding.Resource)
			buf.WriteByte('\n')
			buf.WriteString(fmt.Sprintf("Line: %d\n", finding.Line))
			buf.WriteString(fmt.Sprintf("Rule: %s\n", finding.Rule))
			buf.WriteString("Evidence: ")
			buf.WriteString(finding.Evidence)
			buf.WriteString("\n\n")
		}
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

type jsonReport struct {
	GeneratedAt time.Time `json:"generated_at"`
	Rules       []string  `json:"rules"`
	Findings    []Finding `json:"findings"`
}

// WriteJSON serialises gf findings to JSON.
func WriteJSON(path string, generatedAt time.Time, rules []string, findings []Finding) error {
	report := jsonReport{
		GeneratedAt: generatedAt,
		Rules:       rules,
		Findings:    findings,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	return os.WriteFile(path, data, 0o644)
}
