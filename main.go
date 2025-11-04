package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/config"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/gf"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/input"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/model"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/network"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/output"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/parser"
)

const (
	// RecursionDisabled indicates that recursion is turned off
	RecursionDisabled = 0
	// RecursionUnlimited indicates unlimited recursion depth
	RecursionUnlimited = -1
)

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		exitWithError(err)
	}

	var (
		mode          output.Mode
		htmlPath      string
		jsonPath      string
		rawPath       string
		hasJSONOutput bool
		hasRawOutput  bool
	)

	for _, target := range cfg.Outputs {
		switch target.Format {
		case config.OutputCLI:
			mode |= output.ModeCLI
		case config.OutputHTML:
			mode |= output.ModeHTML
			htmlPath = target.Path
		case config.OutputJSON:
			jsonPath = target.Path
			hasJSONOutput = true
		case config.OutputRaw:
			rawPath = target.Path
			hasRawOutput = true
		}
	}

	// Only default to CLI if no other outputs are specified
	if mode == 0 && !hasJSONOutput && !hasRawOutput {
		mode = output.ModeCLI
	}

	targets, err := input.ResolveTargets(cfg)
	if err != nil {
		exitWithError(err)
	}

	var filterRegex *regexp.Regexp
	if cfg.Regex != "" {
		filterRegex, err = regexp.Compile(cfg.Regex)
		if err != nil {
			exitWithError(fmt.Errorf("invalid regex provided: %w", err))
		}
	}

	endpointRegex := parser.EndpointRegex()

	generatedAt := time.Now()

	var htmlBuilder *strings.Builder
	if mode.Includes(output.ModeHTML) {
		htmlBuilder = &strings.Builder{}
	}

	reports := make([]output.ResourceReport, 0, len(targets))
	var reportsMu sync.Mutex
	var outputMu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tasks := make(chan resourceTask, cfg.Workers)
	var taskWg sync.WaitGroup
	var workerWg sync.WaitGroup

	var firstErr error
	var errOnce sync.Once
	recordError := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	enqueue := func(task resourceTask) {
		if ctx.Err() != nil {
			return
		}

		taskWg.Add(1)
		select {
		case tasks <- task:
			return
		case <-ctx.Done():
			taskWg.Done()
			return
		default:
		}

		go func() {
			select {
			case tasks <- task:
			case <-ctx.Done():
				taskWg.Done()
			}
		}()
	}

	for i := 0; i < cfg.Workers; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for task := range tasks {
				if ctx.Err() != nil {
					taskWg.Done()
					continue
				}

				if task.fromDomain {
					fmt.Printf("Running against: %s\n\n", task.target.URL)
				}

				content, err := resolveContent(ctx, task.target, cfg)
				if err != nil {
					if network.IsTimeoutError(err) {
						fmt.Printf("Request timed out for: %s\n", task.target.URL)
						taskWg.Done()
						continue
					}

					if network.IsDNSOrNetworkError(err) {
						fmt.Printf("DNS or network error for: %s (host may not exist or be unreachable)\n", task.target.URL)
						taskWg.Done()
						continue
					}

					if task.fromDomain {
						fmt.Printf("Invalid input defined or SSL error for: %s\n", task.target.URL)
						taskWg.Done()
						continue
					}

					recordError(fmt.Errorf("invalid input defined or SSL error: %w", err))
					taskWg.Done()
					continue
				}

				endpoints := parser.FindEndpoints(content, endpointRegex, mode.Includes(output.ModeHTML), filterRegex, true)
				report := output.ResourceReport{Resource: task.target.URL, Endpoints: endpoints}

				outputMu.Lock()
				render(mode, report, htmlBuilder)
				outputMu.Unlock()

				reportsMu.Lock()
				reports = append(reports, report)
				reportsMu.Unlock()

				if cfg.Recursive != RecursionDisabled && task.visited != nil {
					processDiscoveredResources(ctx, cfg, task.target.URL, endpoints, task.visited, enqueue, task.depth)
				}

				taskWg.Done()
			}
		}()
	}

	for _, t := range targets {
		// Initialize depth based on recursive mode
		depth := RecursionDisabled
		if cfg.Recursive == RecursionUnlimited {
			depth = RecursionUnlimited
		} else if cfg.Recursive > RecursionDisabled {
			depth = cfg.Recursive
		}

		// Detect resource type from the initial target URL
		rtype := network.DetectResourceType(t.URL)

		task := resourceTask{
			target: t,
			depth:  depth,
			rtype:  rtype,
		}
		if cfg.Recursive != RecursionDisabled {
			task.visited = newVisitedSet()
		}
		enqueue(task)
	}

	go func() {
		taskWg.Wait()
		close(tasks)
	}()

	workerWg.Wait()

	if firstErr != nil {
		exitWithError(firstErr)
	}

	meta := output.BuildMetadata(reports, generatedAt)

	// Process GF pattern matching if enabled
	var gfRules []string
	var gfFindings []output.GFFinding

	if cfg.GFAll || len(cfg.GFPatterns) > 0 {
		definitions, err := gf.LoadDefinitions(cfg.GFPatterns, cfg.GFAll, cfg.GFPath)
		if err != nil {
			exitWithError(fmt.Errorf("unable to load gf rules: %w", err))
		}

		rawFindings := gf.FindInReports(reports, definitions)
		gfRules = gf.RuleNames(definitions)

		// Convert gf.Finding to output.GFFinding
		gfFindings = make([]output.GFFinding, len(rawFindings))
		for i, finding := range rawFindings {
			gfFindings[i] = output.GFFinding{
				Resource: finding.Resource,
				Line:     finding.Line,
				Evidence: finding.Evidence,
				Context:  finding.Context,
				Rules:    finding.Rules,
			}
		}
	}

	// Write outputs
	if rawPath != "" {
		if err := output.WriteRaw(rawPath, reports, meta); err != nil {
			exitWithError(fmt.Errorf("unable to write raw output: %w", err))
		}
	}

	if hasJSONOutput {
		// Write JSON to file or stdout (jsonPath can be empty for stdout)
		if err := output.WriteJSON(jsonPath, reports, meta, gfRules, gfFindings); err != nil {
			exitWithError(fmt.Errorf("unable to write JSON output: %w", err))
		}
	}

	if mode.Includes(output.ModeHTML) {
		if err := output.SaveHTML(htmlBuilder.String(), htmlPath, meta, gfRules, gfFindings); err != nil {
			fmt.Fprintf(os.Stderr, "Output can't be saved in %s due to exception: %v\n", htmlPath, err)
			os.Exit(1)
		}
	}

	if mode.Includes(output.ModeCLI) {
		output.PrintSummary(meta)
		if len(gfFindings) > 0 {
			output.PrintGFFindings(gfRules, gfFindings)
		}
	}
}

func resolveContent(ctx context.Context, t model.Target, cfg config.Config) (string, error) {
	if t.Prefetched {
		return t.Content, nil
	}

	if strings.HasPrefix(t.URL, "file://") {
		return input.ResolveFilePath(t.URL)
	}

	return network.Fetch(ctx, t.URL, cfg)
}

// processDiscoveredResources handles recursive processing of discovered endpoints.
// It validates, filters, and enqueues new resources for processing based on the configured recursion depth.
func processDiscoveredResources(ctx context.Context, cfg config.Config, baseResource string, endpoints []model.Endpoint, visited *visitedSet,
	enqueue func(resourceTask), depth int) {
	if visited == nil {
		return
	}

	// Recursion is disabled
	if depth == RecursionDisabled {
		return
	}

	// Calculate next depth level
	nextDepth := depth
	if depth > RecursionDisabled {
		nextDepth = depth - 1
		if nextDepth == RecursionDisabled {
			// This was the last recursion level, print message after processing
			defer fmt.Printf("Maximum recursion depth reached for %s\n", baseResource)
		}
	}
	// depth == RecursionUnlimited stays RecursionUnlimited

	for _, ep := range endpoints {
		if ctx.Err() != nil {
			return
		}

		// Try to resolve the URL as any supported resource type (JavaScript or Sitemap)
		resolved, resourceType, ok := network.ResolveURL(ep.Link, baseResource, network.ResourceJavaScript, network.ResourceSitemap)
		if !ok {
			continue
		}

		// Apply scope filtering if configured
		if cfg.Scope != "" && !network.WithinScope(resolved, cfg.Scope, cfg.ScopeIncludeSubdomains) {
			continue
		}

		// Skip if already visited
		if !visited.Add(resolved) {
			continue
		}

		// Enqueue the resource for processing
		enqueue(resourceTask{
			target:     model.Target{URL: resolved},
			visited:    visited,
			fromDomain: true,
			depth:      nextDepth,
			rtype:      resourceType,
		})
	}
}

func render(mode output.Mode, report output.ResourceReport, builder *strings.Builder) {
	if mode.Includes(output.ModeCLI) {
		output.PrintCLI(report)
	}

	if mode.Includes(output.ModeHTML) && builder != nil {
		output.AppendHTML(builder, report)
	}
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Usage: %s [Options] use -h for help\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

type resourceTask struct {
	target     model.Target
	visited    *visitedSet
	fromDomain bool
	depth      int
	rtype      network.ResourceType
}

type visitedSet struct {
	mu     sync.Mutex
	values map[string]struct{}
}

func newVisitedSet() *visitedSet {
	return &visitedSet{values: make(map[string]struct{})}
}

func (v *visitedSet) Add(value string) bool {
	canonical := canonicalURL(value)
	if canonical == "" {
		canonical = value
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if _, ok := v.values[canonical]; ok {
		return false
	}
	v.values[canonical] = struct{}{}
	return true
}

func canonicalURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	// Ignore query parameters and fragments to avoid revisiting the same
	// JavaScript resource with different cache-busting values.
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""

	return parsed.String()
}
