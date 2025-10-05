package output

import "fmt"

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
