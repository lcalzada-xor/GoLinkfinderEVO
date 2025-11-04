package output

import (
	"fmt"
	"strings"
)

// PrintCLI prints endpoints to stdout in CLI mode.
func PrintCLI(report ResourceReport) {
	fmt.Printf("Resource: %s\n", report.Resource)
	fmt.Printf("  Endpoints discovered: %d\n", len(report.Endpoints))

	if len(report.Endpoints) == 0 {
		fmt.Println("  No endpoints were found.")
		fmt.Println()
		return
	}

	for _, ep := range report.Endpoints {
		fmt.Printf("    - %s\n", ep.Link)
	}

	fmt.Println()
}

// PrintSummary prints an aggregated summary once all resources have been processed.
func PrintSummary(meta Metadata) {
	fmt.Println("Summary")
	fmt.Println("=======")
	fmt.Printf("Resources scanned : %d\n", meta.TotalResources)
	fmt.Printf("Endpoints discovered: %d\n", meta.TotalEndpoints)
}

// PrintGFFindings prints GF pattern matching results to stdout.
func PrintGFFindings(rules []string, findings []GFFinding) {
	if len(findings) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("Pattern Matching Results (GF)")
	fmt.Println("==============================")
	fmt.Printf("Rules applied: %s\n", strings.Join(rules, ", "))
	fmt.Printf("Total findings: %d\n\n", len(findings))

	for _, finding := range findings {
		fmt.Printf("[%s]\n", finding.Resource)
		fmt.Printf("  Line: %d\n", finding.Line)
		fmt.Printf("  Rules: %s\n", strings.Join(finding.Rules, ", "))
		fmt.Printf("  Evidence: %s\n", finding.Evidence)
		if finding.Context != "" {
			fmt.Printf("  Context:\n    %s\n", finding.Context)
		}
		fmt.Println()
	}
}
