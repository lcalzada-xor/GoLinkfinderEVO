package config

import (
	"errors"
	"flag"
	"time"
)

// Config contains runtime configuration provided via flags.
type Config struct {
	Domain  bool
	Input   string
	Output  string
	Regex   string
	Burp    bool
	Cookies string
	Timeout time.Duration
}

// ParseFlags parses CLI flags into a Config value.
func ParseFlags() (Config, error) {
	cfg := Config{}

	flag.BoolVar(&cfg.Domain, "domain", false, "Input a domain to recursively parse all javascript located in a page")
	flag.BoolVar(&cfg.Domain, "d", false, "Input a domain to recursively parse all javascript located in a page")
	flag.StringVar(&cfg.Input, "input", "", "Input a: URL, file or folder. For folders a wildcard can be used (e.g. '/*.js').")
	flag.StringVar(&cfg.Input, "i", "", "Input a: URL, file or folder. For folders a wildcard can be used (e.g. '/*.js').")
	flag.StringVar(&cfg.Output, "output", "", "Where to save the HTML report, including file name. Leave empty for CLI output.")
	flag.StringVar(&cfg.Output, "o", "", "Where to save the HTML report, including file name. Leave empty for CLI output.")
	flag.StringVar(&cfg.Regex, "regex", "", "RegEx for filtering purposes against found endpoint (e.g. ^/api/)")
	flag.StringVar(&cfg.Regex, "r", "", "RegEx for filtering purposes against found endpoint (e.g. ^/api/)")
	flag.BoolVar(&cfg.Burp, "burp", false, "")
	flag.BoolVar(&cfg.Burp, "b", false, "")
	flag.StringVar(&cfg.Cookies, "cookies", "", "Add cookies for authenticated JS files")
	flag.StringVar(&cfg.Cookies, "c", "", "Add cookies for authenticated JS files")
	timeout := flag.Int("timeout", 10, "How many seconds to wait for the server to send data before giving up")
	flag.IntVar(timeout, "t", 10, "How many seconds to wait for the server to send data before giving up")

	flag.Parse()
	cfg.Timeout = time.Duration(*timeout) * time.Second

	if cfg.Input == "" {
		return cfg, errors.New("-i/--input is required")
	}

	return cfg, nil
}
