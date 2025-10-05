package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/example/GoLinkfinderEVO/internal/config"
	"github.com/example/GoLinkfinderEVO/internal/input"
	"github.com/example/GoLinkfinderEVO/internal/model"
	"github.com/example/GoLinkfinderEVO/internal/network"
	"github.com/example/GoLinkfinderEVO/internal/output"
	"github.com/example/GoLinkfinderEVO/internal/parser"
)

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		exitWithError(err)
	}

	mode := determineMode(cfg.Output)

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

	var htmlBuilder strings.Builder
	var builder *strings.Builder
	if mode == output.ModeHTML {
		builder = &htmlBuilder
	}

	reports := make([]output.ResourceReport, 0, len(targets))

	for _, t := range targets {
		content, err := resolveContent(t, cfg)
		if err != nil {
			exitWithError(fmt.Errorf("invalid input defined or SSL error: %w", err))
		}

		endpoints := parser.FindEndpoints(content, endpointRegex, mode == output.ModeHTML, filterRegex, true)

		report := output.ResourceReport{Resource: t.URL, Endpoints: endpoints}
		render(mode, report, builder)
		reports = append(reports, report)

		if cfg.Domain {
			visited := map[string]struct{}{}
			processDomain(cfg, t.URL, endpoints, endpointRegex, filterRegex, mode, builder, &reports, visited)
		}
	}

	meta := output.BuildMetadata(reports, generatedAt)

	if cfg.Raw != "" {
		if err := output.WriteRaw(cfg.Raw, reports, meta); err != nil {
			exitWithError(fmt.Errorf("unable to write raw output: %w", err))
		}
	}

	if cfg.JSON != "" {
		if err := output.WriteJSON(cfg.JSON, reports, meta); err != nil {
			exitWithError(fmt.Errorf("unable to write JSON output: %w", err))
		}
	}

	if mode == output.ModeHTML {
		if err := output.SaveHTML(htmlBuilder.String(), cfg.Output, meta); err != nil {
			fmt.Fprintf(os.Stderr, "Output can't be saved in %s due to exception: %v\n", cfg.Output, err)
			os.Exit(1)
		}
		return
	}

	output.PrintSummary(meta)
}

func determineMode(outputFlag string) output.Mode {
	if outputFlag == "" || strings.EqualFold(outputFlag, "cli") {
		return output.ModeCLI
	}
	return output.ModeHTML
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

func processDomain(cfg config.Config, baseResource string, endpoints []model.Endpoint, regex *regexp.Regexp, filter *regexp.Regexp,
	mode output.Mode, builder *strings.Builder, reports *[]output.ResourceReport, visited map[string]struct{}) {
	for _, ep := range endpoints {
		resolved, ok := network.CheckURL(ep.Link, baseResource)
		if !ok {
			continue
		}

		if cfg.Scope != "" && !network.WithinScope(resolved, cfg.Scope) {
			continue
		}

		if visited != nil {
			if _, seen := visited[resolved]; seen {
				continue
			}
			visited[resolved] = struct{}{}
		}

		fmt.Printf("Running against: %s\n\n", resolved)
		body, err := network.Fetch(resolved, cfg)
		if err != nil {
			fmt.Printf("Invalid input defined or SSL error for: %s\n", resolved)
			continue
		}

		newEndpoints := parser.FindEndpoints(body, regex, mode == output.ModeHTML, filter, true)
		report := output.ResourceReport{Resource: resolved, Endpoints: newEndpoints}
		render(mode, report, builder)
		if reports != nil {
			*reports = append(*reports, report)
		}

		if len(newEndpoints) > 0 {
			processDomain(cfg, resolved, newEndpoints, regex, filter, mode, builder, reports, visited)
		}
	}
}

func render(mode output.Mode, report output.ResourceReport, builder *strings.Builder) {
	if mode == output.ModeCLI {
		output.PrintCLI(report)
		return
	}

	output.AppendHTML(builder, report)
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Usage: %s [Options] use -h for help\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
