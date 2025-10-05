package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

// Config contains runtime configuration provided via flags.
type Config struct {
	Domain  bool
	Scope   string
	Input   string
	Output  string
	Raw     string
	JSON    string
	Regex   string
	Burp    bool
	Cookies string
	Timeout time.Duration
}

// ParseFlags parses CLI flags into a Config value.
func ParseFlags() (Config, error) {
	cfg := Config{Timeout: 10 * time.Second}

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintln(out, "Options:")

		printOption(out, "domain", "d", "", "Recursively parse JavaScript resources discovered on the provided domain.", "")
		printOption(out, "scope", "s", "string", "Restrict recursive JavaScript fetching to the specified domain (e.g. example.com).", "")
		printOption(out, "input", "i", "string", "URL, file or folder to analyse. For folders you can use wildcards (e.g. '/*.js').", "")
		printOption(out, "output", "o", "string", "Save the HTML report to this path. Leave empty for CLI output.", "")
		printOption(out, "raw", "", "string", "Write the extracted endpoints to a plaintext file.", "")
		printOption(out, "json", "", "string", "Write the report metadata and resources to a JSON file.", "")
		printOption(out, "regex", "r", "string", "Only report endpoints matching the provided regular expression (e.g. '^/api/').", "")
		printOption(out, "burp", "b", "", "Treat the input as a Burp Suite XML export.", "")
		printOption(out, "cookies", "c", "string", "Include cookies when fetching authenticated JavaScript files.", "")
		printOption(out, "timeout", "t", "duration", "Maximum time to wait for server responses (e.g. 10s, 1m).", cfg.Timeout.String())
	}

	flag.BoolVar(&cfg.Domain, "domain", false, "Recursively parse JavaScript resources discovered on the provided domain.")
	registerBoolAlias("d", "domain", &cfg.Domain)

	flag.StringVar(&cfg.Scope, "scope", "", "Restrict recursive JavaScript fetching to the specified domain (e.g. example.com).")
	registerStringAlias("s", "scope", &cfg.Scope)

	flag.StringVar(&cfg.Input, "input", "", "URL, file or folder to analyse. For folders you can use wildcards (e.g. '/*.js').")
	registerStringAlias("i", "input", &cfg.Input)

	flag.StringVar(&cfg.Output, "output", "", "Save the HTML report to this path. Leave empty for CLI output.")
	registerStringAlias("o", "output", &cfg.Output)

	flag.StringVar(&cfg.Raw, "raw", "", "Write the extracted endpoints to a plaintext file.")
	registerStringAlias("raw-output", "raw", &cfg.Raw)

	flag.StringVar(&cfg.JSON, "json", "", "Write the report metadata and resources to a JSON file.")

	flag.StringVar(&cfg.Regex, "regex", "", "Only report endpoints matching the provided regular expression (e.g. '^/api/').")
	registerStringAlias("r", "regex", &cfg.Regex)

	flag.BoolVar(&cfg.Burp, "burp", false, "Treat the input as a Burp Suite XML export.")
	registerBoolAlias("b", "burp", &cfg.Burp)

	flag.StringVar(&cfg.Cookies, "cookies", "", "Include cookies when fetching authenticated JavaScript files.")
	registerStringAlias("c", "cookies", &cfg.Cookies)

	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "Maximum time to wait for server responses (e.g. 10s, 1m).")
	registerDurationAlias("t", "timeout", &cfg.Timeout)

	flag.Parse()

	if cfg.Input == "" {
		return cfg, errors.New("-i/--input is required")
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
