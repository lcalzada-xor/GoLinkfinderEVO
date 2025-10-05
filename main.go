package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/config"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/input"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/model"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/network"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/output"
	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/parser"
)

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		exitWithError(err)
	}

	var mode output.Mode
	var htmlPath string
	var jsonPath string
	var rawPath string

	for _, target := range cfg.Outputs {
		switch target.Format {
		case config.OutputCLI:
			mode |= output.ModeCLI
		case config.OutputHTML:
			mode |= output.ModeHTML
			htmlPath = target.Path
		case config.OutputJSON:
			jsonPath = target.Path
		case config.OutputRaw:
			rawPath = target.Path
		}
	}

	if mode == 0 {
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

				content, err := resolveContent(task.target, cfg)
				if err != nil {
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

				if cfg.Domain && task.visited != nil {
					processDomain(ctx, cfg, task.target.URL, endpoints, task.visited, enqueue, task.depth)
				}

				taskWg.Done()
			}
		}()
	}

	for _, t := range targets {
		task := resourceTask{target: t, depth: cfg.MaxDepth}
		if cfg.Domain {
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

	if rawPath != "" {
		if err := output.WriteRaw(rawPath, reports, meta); err != nil {
			exitWithError(fmt.Errorf("unable to write raw output: %w", err))
		}
	}

	if jsonPath != "" {
		if err := output.WriteJSON(jsonPath, reports, meta); err != nil {
			exitWithError(fmt.Errorf("unable to write JSON output: %w", err))
		}
	}

	if mode.Includes(output.ModeHTML) {
		if err := output.SaveHTML(htmlBuilder.String(), htmlPath, meta); err != nil {
			fmt.Fprintf(os.Stderr, "Output can't be saved in %s due to exception: %v\n", htmlPath, err)
			os.Exit(1)
		}
	}

	if mode.Includes(output.ModeCLI) {
		output.PrintSummary(meta)
	}
}

func resolveContent(t model.Target, cfg config.Config) (string, error) {
	if t.Prefetched {
		return t.Content, nil
	}

	if strings.HasPrefix(t.URL, "file://") {
		return input.ResolveFilePath(t.URL)
	}

	return network.Fetch(t.URL, cfg)
}

func processDomain(ctx context.Context, cfg config.Config, baseResource string, endpoints []model.Endpoint, visited *visitedSet,
	enqueue func(resourceTask), depth int) {
	if visited == nil {
		return
	}

	if cfg.MaxDepth > 0 {
		if depth <= 0 {
			fmt.Printf("Maximum depth (%d) reached for %s\n", cfg.MaxDepth, baseResource)
			return
		}
		depth--
	}

	for _, ep := range endpoints {
		if ctx.Err() != nil {
			return
		}

		resolved, ok := network.CheckURL(ep.Link, baseResource)
		if !ok {
			continue
		}

		if cfg.Scope != "" && !network.WithinScope(resolved, cfg.Scope, cfg.ScopeIncludeSubdomains) {
			continue
		}

		if !visited.Add(resolved) {
			continue
		}

		enqueue(resourceTask{
			target:     model.Target{URL: resolved},
			visited:    visited,
			fromDomain: true,
			depth:      depth,
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
}

type visitedSet struct {
	mu     sync.Mutex
	values map[string]struct{}
}

func newVisitedSet() *visitedSet {
	return &visitedSet{values: make(map[string]struct{})}
}

func (v *visitedSet) Add(value string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, ok := v.values[value]; ok {
		return false
	}
	v.values[value] = struct{}{}
	return true
}
