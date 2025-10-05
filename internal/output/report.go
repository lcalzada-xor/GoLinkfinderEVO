package output

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/GoLinkfinderEVO/internal/model"
)

// ResourceReport describes the endpoints discovered for a single resource.
type ResourceReport struct {
	Resource  string
	Endpoints []model.Endpoint
}

// EndpointCount returns the number of endpoints discovered for the resource.
func (r ResourceReport) EndpointCount() int {
	return len(r.Endpoints)
}

// Metadata captures aggregated information about a run.
type Metadata struct {
	GeneratedAt    time.Time
	TotalResources int
	TotalEndpoints int
}

// BuildMetadata creates a Metadata value from the provided reports.
func BuildMetadata(reports []ResourceReport, generatedAt time.Time) Metadata {
	return Metadata{
		GeneratedAt:    generatedAt,
		TotalResources: len(reports),
		TotalEndpoints: TotalEndpoints(reports),
	}
}

// TotalEndpoints counts the endpoints across all reports.
func TotalEndpoints(reports []ResourceReport) int {
	total := 0
	for _, report := range reports {
		total += len(report.Endpoints)
	}
	return total
}

// WriteRaw writes the discovered endpoints to a plaintext file.
func WriteRaw(path string, reports []ResourceReport, meta Metadata) error {
	var buf bytes.Buffer

	buf.WriteString("# GoLinkfinderEVO raw results\n")
	buf.WriteString(fmt.Sprintf("# Generated at: %s\n", meta.GeneratedAt.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("# Resources scanned: %d\n", meta.TotalResources))
	buf.WriteString(fmt.Sprintf("# Total endpoints: %d\n\n", meta.TotalEndpoints))

	for _, report := range reports {
		buf.WriteString("[Resource] ")
		buf.WriteString(report.Resource)
		buf.WriteByte('\n')

		if len(report.Endpoints) == 0 {
			buf.WriteString("#   No endpoints were found.\n\n")
			continue
		}

		for _, ep := range report.Endpoints {
			buf.WriteString(ep.Link)
			buf.WriteByte('\n')

			trimmed := strings.TrimSpace(ep.Context)
			if trimmed != "" {
				for _, line := range strings.Split(trimmed, "\n") {
					buf.WriteString("#   ")
					buf.WriteString(line)
					buf.WriteByte('\n')
				}
			}

			buf.WriteByte('\n')
		}
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}
