package output

import (
	"bytes"
	"fmt"
	htmlstd "html"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lcalzada-xor/GoLinkfinderEVO/internal/parser"
)

// AppendHTML appends the HTML representation of the endpoints for a resource.
func AppendHTML(builder *strings.Builder, report ResourceReport) {
	if builder == nil {
		return
	}

	escapedURL := htmlstd.EscapeString(report.Resource)

	builder.WriteString("\n        <section class=\"resource\">")
	builder.WriteString("\n            <header class=\"resource-header\">")
	builder.WriteString("\n                <h2 class=\"resource-title\"><a href=\"")
	builder.WriteString(escapedURL)
	builder.WriteString("\" target=\"_blank\" rel=\"nofollow noopener noreferrer\">")
	builder.WriteString(escapedURL)
	builder.WriteString("</a></h2>")
	builder.WriteString("\n                <span class=\"badge\">")
	count := report.EndpointCount()
	builder.WriteString(fmt.Sprintf("%d endpoint", count))
	if count != 1 {
		builder.WriteString("s")
	}
	builder.WriteString("</span>")
	builder.WriteString("\n            </header>")

	if len(report.Endpoints) == 0 {
		builder.WriteString("\n            <p class=\"resource-empty\">No endpoints were found for this resource.</p>")
		builder.WriteString("\n        </section>")
		return
	}

	builder.WriteString("\n            <ul class=\"endpoint-list\">")
	for idx, ep := range report.Endpoints {
		safeLink := htmlstd.EscapeString(ep.Link)
		builder.WriteString("\n                <li class=\"endpoint-item\">")
		builder.WriteString("\n                    <div class=\"endpoint-header\">")
		builder.WriteString(fmt.Sprintf("\n                        <span class=\"endpoint-index\">#%d</span>", idx+1))
		builder.WriteString("\n                        <a href=\"")
		builder.WriteString(safeLink)
		builder.WriteString("\" class=\"endpoint-link\" target=\"_blank\" rel=\"nofollow noopener noreferrer\">")
		builder.WriteString(safeLink)
		builder.WriteString("</a>")
		if ep.Line > 0 {
			builder.WriteString("\n                        <span class=\"endpoint-line\">Line ")
			builder.WriteString(strconv.Itoa(ep.Line))
			builder.WriteString("</span>")
		}

		builder.WriteString("\n                        <button type=\"button\" class=\"copy-button\" data-copy=\"")
		builder.WriteString(safeLink)
		builder.WriteString("\">Copy</button>")
		builder.WriteString("\n                    </div>")

		if ep.Context != "" {
			builder.WriteString("\n                    <pre class=\"endpoint-context\"><code>")
			builder.WriteString(parser.HighlightContext(ep.Context, ep.Link))
			builder.WriteString("</code></pre>")
		}

		builder.WriteString("\n                </li>")
	}
	builder.WriteString("\n            </ul>")
	builder.WriteString("\n        </section>")
}

// SaveHTML renders the final HTML report to the provided output path.
func SaveHTML(content, outputPath string, meta Metadata) error {
	tpl, err := template.New("output").Parse(templateHTML)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	data := struct {
		Content        template.HTML
		GeneratedAt    string
		TotalResources int
		TotalEndpoints int
		HasResults     bool
	}{
		Content:        template.HTML(content),
		GeneratedAt:    meta.GeneratedAt.Format(time.RFC1123),
		TotalResources: meta.TotalResources,
		TotalEndpoints: meta.TotalEndpoints,
		HasResults:     meta.TotalResources > 0,
	}

	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}

	if dir := filepath.Dir(outputPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return err
	}

	abs, err := filepath.Abs(outputPath)
	if err == nil {
		fmt.Printf("URL to access output: file://%s\n", abs)
		openInBrowser(abs)
	}

	return nil
}

func openInBrowser(path string) {
	fileURL := "file://" + path
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", fileURL)
	case "darwin":
		cmd = exec.Command("open", fileURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", fileURL)
	default:
		return
	}

	_ = cmd.Start()
}
