package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

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

	var outputBuilder strings.Builder

	for _, t := range targets {
		content, err := resolveContent(t, cfg)
		if err != nil {
			exitWithError(fmt.Errorf("invalid input defined or SSL error: %w", err))
		}

		endpoints := parser.FindEndpoints(content, endpointRegex, mode == output.ModeHTML, filterRegex, true)

		if cfg.Domain {
			visited := map[string]struct{}{}
			processDomain(cfg, t.URL, endpoints, endpointRegex, filterRegex, mode, &outputBuilder, visited)
		}

		render(mode, t.URL, endpoints, &outputBuilder)
	}

	if mode == output.ModeHTML {
		if err := output.SaveHTML(outputBuilder.String(), cfg.Output); err != nil {
			fmt.Fprintf(os.Stderr, "Output can't be saved in %s due to exception: %v\n", cfg.Output, err)
			os.Exit(1)
		}
	}
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
	mode output.Mode, builder *strings.Builder, visited map[string]struct{}) {
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
		render(mode, resolved, newEndpoints, builder)

		if len(newEndpoints) > 0 {
			processDomain(cfg, resolved, newEndpoints, regex, filter, mode, builder, visited)
		}
	}
}

func render(mode output.Mode, resource string, endpoints []model.Endpoint, builder *strings.Builder) {
	if mode == output.ModeCLI {
		output.PrintCLI(resource, endpoints)
		return
	}

	output.AppendHTML(builder, resource, endpoints)
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Usage: %s [Options] use -h for help\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
