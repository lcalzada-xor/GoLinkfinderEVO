package output

import (
	"fmt"

	"github.com/example/GoLinkfinderEVO/internal/model"
)

// PrintCLI prints endpoints to stdout in CLI mode.
func PrintCLI(resource string, endpoints []model.Endpoint) {
	fmt.Printf("File: %s\n", resource)

	if len(endpoints) == 0 {
		fmt.Println("  No endpoints were found.")
		fmt.Println()
		return
	}

	for _, ep := range endpoints {
		fmt.Printf("  %s\n", ep.Link)
	}

	fmt.Println()
}
