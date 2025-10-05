package main

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"
	"time"

	jsbeautifier "github.com/ditashi/jsbeautifier-go/jsbeautifier"
	"github.com/ditashi/jsbeautifier-go/optargs"
)

const regexStr = `

  (?:"|')

  (
    ((?:[a-zA-Z]{1,10}://|//)
    [^"'/]{1,}\.
    [a-zA-Z]{2,}[^"']{0,})

    |

    ((?:/|\.\./|\./)
    [^"'><,;| *()(%%$^/\\\\\[\]]
    [^"'><,;|()]{1,})

    |

    ([a-zA-Z0-9_\-/]{1,}/
    [a-zA-Z0-9_\-/.]{1,}
    \.(?:[a-zA-Z]{1,4}|action)
    (?:[\?|#][^"|']{0,}|))

    |

    ([a-zA-Z0-9_\-/]{1,}/
    [a-zA-Z0-9_\-/]{3,}
    (?:[\?|#][^"|']{0,}|))

    |

    ([a-zA-Z0-9_\-]{1,}
    \.(?:php|asp|aspx|jsp|json|
         action|html|js|txt|xml)
    (?:[\?|#][^"|']{0,}|))

  )

  (?:"|')

`

const contextDelimiter = "\n"

type config struct {
	domain  bool
	input   string
	output  string
	regex   string
	burp    bool
	cookies string
	timeout time.Duration
}

type target struct {
	url        string
	content    string
	prefetched bool
}

type burpItem struct {
	URL      string `xml:"url"`
	Response struct {
		Text string `xml:",chardata"`
	} `xml:"response"`
}

type burpDocument struct {
	Items []burpItem `xml:"item"`
}

type endpoint struct {
	Link    string
	Context string
}

func main() {
	cfg := parseFlags()
	if cfg.input == "" {
		parserError("input is required")
	}

	cfg.input = strings.TrimSuffix(cfg.input, "/")

	mode := 1
	if cfg.output == "cli" {
		mode = 0
	}

	targets, err := parserInput(cfg)
	if err != nil {
		parserError(err.Error())
	}

	var filterRegex *regexp.Regexp
	if cfg.regex != "" {
		filterRegex, err = regexp.Compile(cfg.regex)
		if err != nil {
			parserError(fmt.Sprintf("invalid regex provided: %v", err))
		}
	}

	endpointRegex := regexp.MustCompile(regexStr)

	var outputBuilder strings.Builder

	for _, t := range targets {
		var content string
		currentURL := t.url
		if t.prefetched {
			content = t.content
		} else {
			body, err := sendRequest(currentURL, cfg)
			if err != nil {
				parserError(fmt.Sprintf("invalid input defined or SSL error: %v", err))
			}
			content = body
		}

		endpoints := parseFile(content, endpointRegex, mode == 1, filterRegex, true)

		if cfg.domain {
			for _, ep := range endpoints {
				resolved, ok := checkURL(ep.Link, cfg.input)
				if !ok {
					continue
				}

				fmt.Printf("Running against: %s\n\n", resolved)
				body, err := sendRequest(resolved, cfg)
				if err != nil {
					fmt.Printf("Invalid input defined or SSL error for: %s\n", resolved)
					continue
				}

				newEndpoints := parseFile(body, endpointRegex, mode == 1, filterRegex, true)
				if cfg.output == "cli" {
					cliOutput(newEndpoints)
				} else {
					appendHTML(&outputBuilder, resolved, newEndpoints)
				}
			}
		}

		if cfg.output == "cli" {
			cliOutput(endpoints)
		} else {
			appendHTML(&outputBuilder, currentURL, endpoints)
		}
	}

	if cfg.output != "cli" {
		if err := saveHTML(outputBuilder.String(), cfg.output); err != nil {
			fmt.Fprintf(os.Stderr, "Output can't be saved in %s due to exception: %v\n", cfg.output, err)
			os.Exit(1)
		}
	}
}

func parseFlags() config {
	cfg := config{}

	flag.BoolVar(&cfg.domain, "domain", false, "Input a domain to recursively parse all javascript located in a page")
	flag.BoolVar(&cfg.domain, "d", false, "Input a domain to recursively parse all javascript located in a page")
	flag.StringVar(&cfg.input, "input", "", "Input a: URL, file or folder. For folders a wildcard can be used (e.g. '/*.js').")
	flag.StringVar(&cfg.input, "i", "", "Input a: URL, file or folder. For folders a wildcard can be used (e.g. '/*.js').")
	flag.StringVar(&cfg.output, "output", "output.html", "Where to save the file, including file name. Default: output.html")
	flag.StringVar(&cfg.output, "o", "output.html", "Where to save the file, including file name. Default: output.html")
	flag.StringVar(&cfg.regex, "regex", "", "RegEx for filtering purposes against found endpoint (e.g. ^/api/)")
	flag.StringVar(&cfg.regex, "r", "", "RegEx for filtering purposes against found endpoint (e.g. ^/api/)")
	flag.BoolVar(&cfg.burp, "burp", false, "")
	flag.BoolVar(&cfg.burp, "b", false, "")
	flag.StringVar(&cfg.cookies, "cookies", "", "Add cookies for authenticated JS files")
	flag.StringVar(&cfg.cookies, "c", "", "Add cookies for authenticated JS files")
	timeout := flag.Int("timeout", 10, "How many seconds to wait for the server to send data before giving up")
	flag.IntVar(timeout, "t", 10, "How many seconds to wait for the server to send data before giving up")

	flag.Parse()
	cfg.timeout = time.Duration(*timeout) * time.Second

	if cfg.input == "" {
		parserError("-i/--input is required")
	}

	return cfg
}

func parserError(message string) {
	fmt.Fprintf(os.Stderr, "Usage: %s [Options] use -h for help\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	os.Exit(1)
}

func parserInput(cfg config) ([]target, error) {
	input := cfg.input

	if strings.HasPrefix(input, "view-source:") {
		input = input[12:]
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") ||
		strings.HasPrefix(input, "file://") || strings.HasPrefix(input, "ftp://") ||
		strings.HasPrefix(input, "ftps://") {
		if cfg.burp {
			return nil, errors.New("burp mode requires a file input")
		}
		return []target{{url: input}}, nil
	}

	if cfg.burp {
		return parseBurpFile(input)
	}

	if strings.Contains(input, "*") {
		matches, err := filepath.Glob(input)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, errors.New("input with wildcard does not match any files")
		}
		var targets []target
		for _, path := range matches {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				abs, err := filepath.Abs(path)
				if err != nil {
					return nil, err
				}
				targets = append(targets, target{url: "file://" + abs})
			}
		}
		if len(targets) == 0 {
			return nil, errors.New("input with wildcard does not match any files")
		}
		return targets, nil
	}

	if _, err := os.Stat(input); err == nil {
		abs, err := filepath.Abs(input)
		if err != nil {
			return nil, err
		}
		return []target{{url: "file://" + abs}}, nil
	}

	return nil, errors.New("file could not be found (maybe you forgot to add http/https)")
}

func parseBurpFile(path string) ([]target, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc burpDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	var targets []target
	for _, item := range doc.Items {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(item.Response.Text))
		if err != nil {
			return nil, err
		}
		targets = append(targets, target{url: item.URL, content: string(decoded), prefetched: true})
	}

	return targets, nil
}

func sendRequest(rawURL string, cfg config) (string, error) {
	if strings.HasPrefix(rawURL, "file://") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", err
		}
		path, err := url.PathUnescape(u.Path)
		if err != nil {
			return "", err
		}
		if path == "" {
			path = strings.TrimPrefix(rawURL, "file://")
		}
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = strings.TrimPrefix(path, "/")
		}
		path = filepath.FromSlash(path)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	client := &http.Client{Timeout: cfg.timeout}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	if cfg.cookies != "" {
		req.Header.Set("Cookie", cfg.cookies)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		defer gz.Close()
		reader = gz
	case "deflate":
		fl := flate.NewReader(resp.Body)
		defer fl.Close()
		reader = fl
	default:
		reader = resp.Body
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func parseFile(content string, regex *regexp.Regexp, includeContext bool, filter *regexp.Regexp, noDup bool) []endpoint {
	processed := content
	if includeContext {
		processed = beautify(content)
	}

	matches := regex.FindAllStringSubmatchIndex(processed, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	var results []endpoint

	for _, idx := range matches {
		if len(idx) < 4 {
			continue
		}
		linkStart, linkEnd := idx[2], idx[3]
		matchStart, matchEnd := idx[0], idx[1]
		if linkStart < 0 || linkEnd < 0 || linkStart > len(processed) || linkEnd > len(processed) {
			continue
		}
		link := processed[linkStart:linkEnd]
		if filter != nil && !filter.MatchString(link) {
			continue
		}
		if noDup {
			if _, ok := seen[link]; ok {
				continue
			}
			seen[link] = struct{}{}
		}

		ep := endpoint{Link: link}
		if includeContext {
			ep.Context = extractContext(processed, matchStart, matchEnd, false)
		}
		results = append(results, ep)
	}

	return results
}

func extractContext(content string, matchStart, matchEnd int, includeDelimiter bool) string {
	if matchStart < 0 || matchEnd < 0 {
		return ""
	}

	startSection := content[:matchStart]
	endSection := content[matchEnd:]

	prevIdx := strings.LastIndex(startSection, contextDelimiter)
	nextIdx := strings.Index(endSection, contextDelimiter)

	start := 0
	if prevIdx != -1 {
		start = prevIdx
		if !includeDelimiter {
			start += len(contextDelimiter)
		}
	}

	end := len(content)
	if nextIdx != -1 {
		end = matchEnd + nextIdx
		if includeDelimiter {
			end += len(contextDelimiter)
		}
	}

	if start > len(content) {
		start = len(content)
	}
	if end > len(content) {
		end = len(content)
	}
	if start > end {
		start, end = end, start
	}

	return content[start:end]
}

func beautify(content string) string {
	if len(content) > 1_000_000 {
		replacer := strings.NewReplacer(";", ";\r\n", ",", ",\r\n")
		return replacer.Replace(content)
	}

	options := optargs.MapType{}
	options.Copy(optargs.MapType(jsbeautifier.DefaultOptions()))
	result, err := jsbeautifier.Beautify(&content, options)
	if err != nil {
		return content
	}
	return result
}

func cliOutput(endpoints []endpoint) {
	for _, ep := range endpoints {
		fmt.Println(html.EscapeString(ep.Link))
	}
}

func appendHTML(builder *strings.Builder, resource string, endpoints []endpoint) {
	escapedURL := html.EscapeString(resource)
	builder.WriteString("\n                <h1>File: <a href=\"")
	builder.WriteString(escapedURL)
	builder.WriteString("\" target=\"_blank\" rel=\"nofollow noopener noreferrer\">")
	builder.WriteString(escapedURL)
	builder.WriteString("</a></h1>\n")

	for _, ep := range endpoints {
		safeLink := html.EscapeString(ep.Link)
		builder.WriteString("                <div><a href=\"")
		builder.WriteString(safeLink)
		builder.WriteString("\" class='text'>")
		builder.WriteString(safeLink)
		builder.WriteString("</a><div class='container'>")
		context := html.EscapeString(ep.Context)
		highlight := strings.ReplaceAll(context, safeLink, "<span style='background-color:yellow'>"+safeLink+"</span>")
		builder.WriteString(highlight)
		builder.WriteString("</div></div>\n")
	}
}

func saveHTML(content, outputPath string) error {
	tplData, err := os.ReadFile("template.html")
	if err != nil {
		return err
	}

	tpl, err := template.New("output").Parse(string(tplData))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, map[string]any{"Content": content}); err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return err
	}

	abs, err := filepath.Abs(outputPath)
	if err == nil {
		fmt.Printf("URL to access output: file://%s\n", abs)
		openInBrowser(abs)
	}

	return nil
}

func openInBrowser(path string) {
	fileURL := "file://" + path
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", fileURL)
	case "darwin":
		cmd = exec.Command("open", fileURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", fileURL)
	default:
		return
	}

	_ = cmd.Start()
}

func checkURL(raw, base string) (string, bool) {
	if !strings.HasSuffix(raw, ".js") {
		return "", false
	}

	parts := strings.Split(raw, "/")
	for _, p := range parts {
		if p == "node_modules" || p == "jquery.js" {
			return "", false
		}
	}

	resolved := raw
	if strings.HasPrefix(resolved, "//") {
		resolved = "https:" + resolved
	} else if !strings.HasPrefix(resolved, "http") {
		if strings.HasPrefix(resolved, "/") {
			resolved = base + resolved
		} else {
			resolved = base + "/" + resolved
		}
	}

	return resolved, true
}
