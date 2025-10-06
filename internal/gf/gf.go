package gf

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/output"
)

// Definition represents a gf rule loaded from disk.
type Definition struct {
	Name     string
	Patterns []*regexp.Regexp
}

// Finding captures a gf match extracted from the reports.
type Finding struct {
	Resource string   `json:"resource"`
	Line     int      `json:"line"`
	Evidence string   `json:"evidence"`
	Context  string   `json:"context,omitempty"`
	Rules    []string `json:"rules"`
}

// LoadDefinitions loads gf rule definitions from the ~/.gf directory.
func LoadDefinitions(names []string, useAll bool) ([]Definition, error) {
	dir, err := defaultDir()
	if err != nil {
		return nil, err
	}

	return loadDefinitionsFromDir(dir, names, useAll)
}

func defaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to resolve user home directory: %w", err)
	}

	dir := filepath.Join(home, ".gf")
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("gf directory not found at %s: %w", dir, err)
	}

	return dir, nil
}

func loadDefinitionsFromDir(dir string, names []string, useAll bool) ([]Definition, error) {
	var files []string
	if useAll {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("unable to read gf directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			files = append(files, filepath.Join(dir, entry.Name()))
		}

		if len(files) == 0 {
			return nil, errors.New("no gf rules found in ~/.gf")
		}

		sort.Strings(files)
	} else {
		if len(names) == 0 {
			return nil, errors.New("no gf rule names provided")
		}

		for _, name := range names {
			if name == "" {
				continue
			}

			filename := name
			if !strings.HasSuffix(filename, ".json") {
				filename += ".json"
			}

			path := filepath.Join(dir, filename)
			if _, err := os.Stat(path); err != nil {
				return nil, fmt.Errorf("gf rule %s not found in %s", filename, dir)
			}
			files = append(files, path)
		}

		if len(files) == 0 {
			return nil, errors.New("no gf rule names provided")
		}
	}

	definitions := make([]Definition, 0, len(files))
	for _, file := range files {
		def, err := parseDefinition(file)
		if err != nil {
			return nil, fmt.Errorf("unable to parse %s: %w", filepath.Base(file), err)
		}
		definitions = append(definitions, def)
	}

	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })

	return definitions, nil
}

type gfFile struct {
	Pattern  string   `json:"pattern"`
	Patterns []string `json:"patterns"`
	Flags    string   `json:"flags"`
}

func parseDefinition(path string) (Definition, error) {
	var def Definition

	content, err := os.ReadFile(path)
	if err != nil {
		return def, err
	}

	var file gfFile
	if err := json.Unmarshal(content, &file); err != nil {
		return def, err
	}

	var rawPatterns []string
	if file.Pattern != "" {
		rawPatterns = append(rawPatterns, file.Pattern)
	}
	rawPatterns = append(rawPatterns, file.Patterns...)

	if len(rawPatterns) == 0 {
		return def, errors.New("gf rule does not define any patterns")
	}

	ignoreCase := strings.Contains(strings.ToLower(file.Flags), "i")

	compiled := make([]*regexp.Regexp, 0, len(rawPatterns))
	for _, raw := range rawPatterns {
		pattern := raw
		if ignoreCase && !strings.HasPrefix(pattern, "(?i)") {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return def, fmt.Errorf("invalid pattern %q: %w", raw, err)
		}
		compiled = append(compiled, re)
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	return Definition{Name: name, Patterns: compiled}, nil
}

// FindInReports runs all gf definitions against the collected reports and returns any matches found.
func FindInReports(reports []output.ResourceReport, defs []Definition) []Finding {
	if len(defs) == 0 {
		return nil
	}

	type key struct {
		resource string
		line     int
		evidence string
	}

	findingsByKey := make(map[key]*Finding)

	for _, report := range reports {
		for _, ep := range report.Endpoints {
			context := strings.TrimSpace(ep.Context)

			for _, def := range defs {
				for _, re := range def.Patterns {
					matches := re.FindAllString(ep.Link, -1)
					if matches == nil {
						continue
					}

					for _, match := range matches {
						k := key{resource: report.Resource, line: ep.Line, evidence: match}

						finding, ok := findingsByKey[k]
						if !ok {
							finding = &Finding{
								Resource: report.Resource,
								Line:     ep.Line,
								Evidence: match,
								Context:  context,
							}
							findingsByKey[k] = finding
						} else if finding.Context == "" && context != "" {
							finding.Context = context
						}

						if !containsRule(finding.Rules, def.Name) {
							finding.Rules = append(finding.Rules, def.Name)
						}
					}
				}
			}
		}
	}

	if len(findingsByKey) == 0 {
		return nil
	}

	findings := make([]Finding, 0, len(findingsByKey))
	for _, finding := range findingsByKey {
		sort.Strings(finding.Rules)
		findings = append(findings, *finding)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Resource != findings[j].Resource {
			return findings[i].Resource < findings[j].Resource
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		if findings[i].Evidence != findings[j].Evidence {
			return findings[i].Evidence < findings[j].Evidence
		}
		return strings.Join(findings[i].Rules, ",") < strings.Join(findings[j].Rules, ",")
	})

	return findings
}

func containsRule(rules []string, rule string) bool {
	for _, existing := range rules {
		if existing == rule {
			return true
		}
	}
	return false
}

// RuleNames extracts the names of the loaded definitions.
func RuleNames(defs []Definition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	sort.Strings(names)
	return names
}
