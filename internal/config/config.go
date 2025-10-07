package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config contains runtime configuration provided via flags.
type Config struct {
	Domain                 bool
	Scope                  string
	Input                  string
	Regex                  string
	Burp                   bool
	Cookies                string
	Proxy                  string
	Insecure               bool
	Timeout                time.Duration
	Workers                int
	MaxDepth               int
	ScopeIncludeSubdomains bool
	Outputs                []OutputTarget
	GFAll                  bool
	GFPatterns             []string
}

// OutputFormat represents a supported output channel.
type OutputFormat int

const (
	OutputCLI OutputFormat = iota
	OutputHTML
	OutputJSON
	OutputRaw
	OutputGFText
	OutputGFJSON
)

func (f OutputFormat) String() string {
	switch f {
	case OutputCLI:
		return "cli"
	case OutputHTML:
		return "html"
	case OutputJSON:
		return "json"
	case OutputRaw:
		return "raw"
	case OutputGFText:
		return "gf.txt"
	case OutputGFJSON:
		return "gf.json"
	default:
		return "unknown"
	}
}

func (f OutputFormat) requiresPath() bool {
	switch f {
	case OutputHTML, OutputJSON, OutputRaw, OutputGFText, OutputGFJSON:
		return true
	default:
		return false
	}
}

// OutputTarget represents a configured output destination.
type OutputTarget struct {
	Format OutputFormat
	Path   string
}

// ParseFlags parses CLI flags into a Config value.
func ParseFlags() (Config, error) {
	defaultWorkers := runtime.NumCPU()
	if defaultWorkers < 1 {
		defaultWorkers = 1
	}

	cfg := Config{Timeout: 10 * time.Second, Workers: defaultWorkers}

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintln(out, "Options:")

		printOption(out, "domain", "d", "", "Recursively parse JavaScript resources discovered on the provided domain.", "")
		printOption(out, "scope", "s", "string", "Restrict recursive JavaScript fetching to the specified domain (e.g. example.com).", "")
		printOption(out, "scope-include-subdomains", "", "", "When used with --scope, also allow subdomains of the provided domain.", "")
		printOption(out, "input", "i", "string", "URL, file or folder to analyse. For folders you can use wildcards (e.g. '/*.js').", "")
		printOption(out, "output", "o", "string", "Configure one or more outputs (e.g. 'cli', 'html=report.html', 'gf.txt=findings.txt'). May be repeated or comma separated.", "cli")
		printOption(out, "regex", "r", "string", "Only report endpoints matching the provided regular expression (e.g. '^/api/').", "")
		printOption(out, "burp", "b", "", "Treat the input as a Burp Suite XML export.", "")
		printOption(out, "cookies", "c", "string", "Include cookies when fetching authenticated JavaScript files.", "")
		printOption(out, "proxy", "", "string", "Forward HTTP requests through the provided proxy (e.g. http://127.0.0.1:8080).", "")
		printOption(out, "insecure", "", "", "Skip TLS certificate verification when fetching HTTPS resources.", "")
		printOption(out, "timeout", "t", "duration", "Maximum time to wait for server responses (e.g. 10s, 1m).", cfg.Timeout.String())
		printOption(out, "workers", "", "int", "Maximum number of concurrent fetch operations.", strconv.Itoa(cfg.Workers))
		printOption(out, "max-depth", "", "int", "Maximum recursion depth when using --domain (0 means unlimited).", strconv.Itoa(cfg.MaxDepth))
		printOption(out, "gf", "", "string", "Comma separated list of gf rules located in ~/.gf or 'all' to run every rule.", "")
	}

	flag.BoolVar(&cfg.Domain, "domain", false, "Recursively parse JavaScript resources discovered on the provided domain.")
	registerBoolAlias("d", "domain", &cfg.Domain)

	flag.StringVar(&cfg.Scope, "scope", "", "Restrict recursive JavaScript fetching to the specified domain (e.g. example.com).")
	registerStringAlias("s", "scope", &cfg.Scope)

	flag.BoolVar(&cfg.ScopeIncludeSubdomains, "scope-include-subdomains", false, "When used with --scope, also allow subdomains of the provided domain.")

	flag.StringVar(&cfg.Input, "input", "", "URL, file or folder to analyse. For folders you can use wildcards (e.g. '/*.js').")
	registerStringAlias("i", "input", &cfg.Input)

	collector := newOutputCollector(&cfg.Outputs)
	flag.Var(collector, "output", "Configure one or more outputs (e.g. cli, html=report.html). May be repeated or comma separated.")
	flag.Var(collector, "o", "Alias for --output.")

	flag.Var(newOutputAlias(collector, OutputRaw), "raw", "Write the extracted endpoints to a plaintext file.")
	flag.Var(newOutputAlias(collector, OutputRaw), "raw-output", "Alias for --raw.")

	flag.Var(newOutputAlias(collector, OutputJSON), "json", "Write the report metadata and resources to a JSON file.")

	flag.StringVar(&cfg.Regex, "regex", "", "Only report endpoints matching the provided regular expression (e.g. '^/api/').")
	registerStringAlias("r", "regex", &cfg.Regex)

	flag.BoolVar(&cfg.Burp, "burp", false, "Treat the input as a Burp Suite XML export.")
	registerBoolAlias("b", "burp", &cfg.Burp)

	flag.StringVar(&cfg.Cookies, "cookies", "", "Include cookies when fetching authenticated JavaScript files.")
	registerStringAlias("c", "cookies", &cfg.Cookies)

	flag.StringVar(&cfg.Proxy, "proxy", "", "Forward HTTP requests through the provided proxy (e.g. http://127.0.0.1:8080).")

	flag.BoolVar(&cfg.Insecure, "insecure", false, "Skip TLS certificate verification when fetching HTTPS resources.")

	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "Maximum time to wait for server responses (e.g. 10s, 1m).")
	registerDurationAlias("t", "timeout", &cfg.Timeout)

	flag.IntVar(&cfg.Workers, "workers", cfg.Workers, "Maximum number of concurrent fetch operations.")
	flag.IntVar(&cfg.MaxDepth, "max-depth", 0, "Maximum recursion depth when using --domain (0 means unlimited).")

	var gfRaw string
	flag.StringVar(&gfRaw, "gf", "", "Comma separated list of gf rules located in ~/.gf or 'all' to run every rule.")

	flag.Parse()

	if !collector.has(OutputCLI) {
		_ = collector.add(OutputCLI, "")
	}

	gfRaw = strings.TrimSpace(gfRaw)
	if gfRaw != "" {
		if strings.EqualFold(gfRaw, "all") {
			cfg.GFAll = true
		} else {
			parts := strings.Split(gfRaw, ",")
			seen := make(map[string]struct{}, len(parts))
			for _, part := range parts {
				name := strings.TrimSpace(part)
				if name == "" {
					continue
				}
				lname := strings.ToLower(name)
				if _, ok := seen[lname]; ok {
					continue
				}
				seen[lname] = struct{}{}
				cfg.GFPatterns = append(cfg.GFPatterns, name)
			}

			if len(cfg.GFPatterns) == 0 {
				return cfg, errors.New("--gf requires at least one rule name or 'all'")
			}
		}
	}

	if cfg.Input == "" {
		return cfg, errors.New("-i/--input is required")
	}

	if cfg.Workers < 1 {
		return cfg, errors.New("--workers must be at least 1")
	}

	if cfg.MaxDepth < 0 {
		return cfg, errors.New("--max-depth must be at least 0")
	}

	return cfg, nil
}

func registerStringAlias(name, canonical string, target *string) {
	flag.CommandLine.Var(&stringAlias{target: target}, name, fmt.Sprintf("Alias for --%s", canonical))
}

func registerBoolAlias(name, canonical string, target *bool) {
	flag.CommandLine.Var(&boolAlias{target: target}, name, fmt.Sprintf("Alias for --%s", canonical))
}

func registerDurationAlias(name, canonical string, target *time.Duration) {
	flag.CommandLine.Var(&durationAlias{target: target}, name, fmt.Sprintf("Alias for --%s", canonical))
}

func printOption(out io.Writer, primary, alias, value, description, defaultValue string) {
	line := fmt.Sprintf("  -%s", primary)
	if alias != "" {
		line += fmt.Sprintf(" (-%s)", alias)
	}
	if value != "" {
		line += " " + value
	}
	if defaultValue != "" {
		line += fmt.Sprintf(" (default %s)", defaultValue)
	}

	fmt.Fprintln(out, line)
	fmt.Fprintf(out, "        %s\n", description)
}

type stringAlias struct {
	target *string
}

func (s *stringAlias) Set(value string) error {
	*s.target = value
	return nil
}

func (s *stringAlias) String() string {
	if s.target == nil {
		return ""
	}
	return *s.target
}

type boolAlias struct {
	target *bool
}

func (b *boolAlias) Set(value string) error {
	if value == "" {
		*b.target = true
		return nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	*b.target = parsed
	return nil
}

func (b *boolAlias) String() string {
	if b.target == nil {
		return "false"
	}
	return strconv.FormatBool(*b.target)
}

func (b *boolAlias) IsBoolFlag() bool {
	return true
}

type durationAlias struct {
	target *time.Duration
}

func (d *durationAlias) Set(value string) error {
	if value == "" {
		return errors.New("duration flag requires a value")
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		seconds, convErr := strconv.Atoi(value)
		if convErr != nil {
			return err
		}
		*d.target = time.Duration(seconds) * time.Second
		return nil
	}

	*d.target = parsed
	return nil
}

func (d *durationAlias) String() string {
	if d.target == nil {
		return ""
	}
	return d.target.String()
}

type outputCollector struct {
	targets  *[]OutputTarget
	selected map[OutputFormat]string
}

func newOutputCollector(targets *[]OutputTarget) *outputCollector {
	return &outputCollector{targets: targets, selected: make(map[OutputFormat]string)}
}

func (o *outputCollector) Set(value string) error {
	if value == "" {
		return errors.New("output flag requires a value")
	}

	entries := strings.Split(value, ",")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		format, path, err := parseOutputEntry(entry)
		if err != nil {
			return err
		}

		if err := o.add(format, path); err != nil {
			return err
		}
	}

	return nil
}

func (o *outputCollector) String() string {
	if o == nil || o.targets == nil {
		return ""
	}

	parts := make([]string, 0, len(*o.targets))
	for _, target := range *o.targets {
		parts = append(parts, formatOutputValue(target))
	}

	return strings.Join(parts, ",")
}

func (o *outputCollector) add(format OutputFormat, path string) error {
	if o == nil {
		return errors.New("output collector not initialised")
	}

	if _, exists := o.selected[format]; exists {
		return fmt.Errorf("output %s already specified", format.String())
	}

	if format.requiresPath() && path == "" {
		return fmt.Errorf("output %s requires a file path", format.String())
	}

	if !format.requiresPath() && path != "" {
		return fmt.Errorf("output %s does not accept a file path", format.String())
	}

	*o.targets = append(*o.targets, OutputTarget{Format: format, Path: path})
	o.selected[format] = path
	return nil
}

func (o *outputCollector) has(format OutputFormat) bool {
	if o == nil {
		return false
	}
	_, ok := o.selected[format]
	return ok
}

func (o *outputCollector) pathFor(format OutputFormat) string {
	if o == nil {
		return ""
	}
	return o.selected[format]
}

func formatOutputValue(target OutputTarget) string {
	switch target.Format {
	case OutputCLI:
		return target.Format.String()
	default:
		return fmt.Sprintf("%s=%s", target.Format.String(), target.Path)
	}
}

func parseOutputEntry(entry string) (OutputFormat, string, error) {
	var formatStr, path string

	if idx := strings.IndexAny(entry, "=:"); idx != -1 {
		formatStr = strings.ToLower(strings.TrimSpace(entry[:idx]))
		path = strings.TrimSpace(entry[idx+1:])
	} else {
		lowered := strings.ToLower(strings.TrimSpace(entry))
		switch lowered {
		case OutputCLI.String(), OutputHTML.String(), OutputJSON.String(), OutputRaw.String():
			formatStr = lowered
		default:
			formatStr = OutputHTML.String()
			path = entry
		}
	}

	format, err := parseOutputFormat(formatStr)
	if err != nil {
		return 0, "", err
	}

	if format.requiresPath() && path == "" {
		return 0, "", fmt.Errorf("output %s requires a file path", format.String())
	}

	if !format.requiresPath() && path != "" {
		return 0, "", fmt.Errorf("output %s does not accept a file path", format.String())
	}

	return format, path, nil
}

func parseOutputFormat(value string) (OutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case OutputCLI.String():
		return OutputCLI, nil
	case OutputHTML.String():
		return OutputHTML, nil
	case OutputJSON.String():
		return OutputJSON, nil
	case OutputRaw.String():
		return OutputRaw, nil
	case OutputGFText.String():
		return OutputGFText, nil
	case OutputGFJSON.String():
		return OutputGFJSON, nil
	default:
		if value == "" {
			return OutputHTML, nil
		}
		return 0, fmt.Errorf("unsupported output format %q", value)
	}
}

type outputAlias struct {
	collector *outputCollector
	format    OutputFormat
}

func newOutputAlias(collector *outputCollector, format OutputFormat) flag.Value {
	return &outputAlias{collector: collector, format: format}
}

func (a *outputAlias) Set(value string) error {
	if a.collector == nil {
		return errors.New("output alias not initialised")
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if a.format.requiresPath() {
			return fmt.Errorf("output %s requires a file path", a.format.String())
		}
		return a.collector.add(a.format, "")
	}

	return a.collector.add(a.format, trimmed)
}

func (a *outputAlias) String() string {
	if a.collector == nil {
		return ""
	}
	return a.collector.pathFor(a.format)
}
