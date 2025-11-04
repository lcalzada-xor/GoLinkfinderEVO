package output

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// GFFinding represents a pattern match found by GF.
type GFFinding struct {
	Resource string   `json:"resource"`
	Line     int      `json:"line"`
	Evidence string   `json:"evidence"`
	Context  string   `json:"context,omitempty"`
	Rules    []string `json:"rules"`
}

// GFReport contains GF pattern matching results.
type GFReport struct {
	Rules    []string    `json:"rules"`
	Total    int         `json:"total"`
	Findings []GFFinding `json:"findings"`
}

type jsonPayload struct {
	Meta       Metadata         `json:"meta"`
	Resources  []ResourceReport `json:"resources"`
	GFFindings *GFReport        `json:"gf_findings,omitempty"`
}

// WriteJSON writes the discovered resources and metadata to a JSON file or stdout.
// If path is empty or "-", writes to stdout. Otherwise writes to the specified file.
func WriteJSON(path string, reports []ResourceReport, meta Metadata, gfRules []string, gfFindings []GFFinding) error {
	payload := jsonPayload{Meta: meta, Resources: reports}

	// Add GF findings if present
	if len(gfFindings) > 0 {
		payload.GFFindings = &GFReport{
			Rules:    gfRules,
			Total:    len(gfFindings),
			Findings: gfFindings,
		}
	}

	// Write to stdout if path is empty or "-"
	if path == "" || path == "-" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	}

	// Write to file
	data, err := json.MarshalIndent(payload, "", "  ")
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
