package output

import (
	"fmt"
	"html"

	"github.com/example/GoLinkfinderEVO/internal/model"
)

// PrintCLI prints endpoints to stdout in CLI mode.
func PrintCLI(endpoints []model.Endpoint) {
	for _, ep := range endpoints {
		fmt.Println(html.EscapeString(ep.Link))
	}
}
