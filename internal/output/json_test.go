package output

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/model"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	reports := []ResourceReport{
		{
			Resource: "https://example.com/app.js",
			Endpoints: []model.Endpoint{
				{
					Link:    "/api/v1",
					Context: "fetch('/api/v1')",
					Line:    12,
				},
			},
		},
		{
			Resource:  "https://example.com/other.js",
			Endpoints: []model.Endpoint{},
		},
	}

	meta := Metadata{
		GeneratedAt:    time.Date(2024, 3, 1, 15, 4, 5, 0, time.UTC),
		TotalResources: len(reports),
		TotalEndpoints: TotalEndpoints(reports),
	}

	if err := WriteJSON(path, reports, meta, nil, nil); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read JSON output: %v", err)
	}

	const expected = `{
  "meta": {
    "GeneratedAt": "2024-03-01T15:04:05Z",
    "TotalResources": 2,
    "TotalEndpoints": 1
  },
  "resources": [
    {
      "Resource": "https://example.com/app.js",
      "Endpoints": [
        {
          "Link": "/api/v1",
          "Context": "fetch('/api/v1')",
          "Line": 12
        }
      ]
    },
    {
      "Resource": "https://example.com/other.js",
      "Endpoints": []
    }
  ]
}`

	if string(data) != expected {
		t.Fatalf("unexpected JSON output:\nexpected:\n%s\n\nactual:\n%s", expected, string(data))
	}
}
