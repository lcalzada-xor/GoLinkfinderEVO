package output

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/example/GoLinkfinderEVO/internal/model"
	"github.com/example/GoLinkfinderEVO/internal/parser"
)

// AppendHTML appends the HTML representation of the endpoints for a resource.
func AppendHTML(builder *strings.Builder, resource string, endpoints []model.Endpoint) {
	escapedURL := html.EscapeString(resource)
	builder.WriteString("\n                <h1>File: <a href=\"")
	builder.WriteString(escapedURL)
	builder.WriteString("\" target=\"_blank\" rel=\"nofollow noopener noreferrer\">")
	builder.WriteString(escapedURL)
	builder.WriteString("</a></h1>\n")

	for _, ep := range endpoints {
		safeLink := html.EscapeString(ep.Link)
		builder.WriteString("                <div><a href=\"")
		builder.WriteString(safeLink)
		builder.WriteString("\" class='text'>")
		builder.WriteString(safeLink)
		builder.WriteString("</a><div class='container'>")
		builder.WriteString(parser.HighlightContext(ep.Context, ep.Link))
		builder.WriteString("</div></div>\n")
	}
}

// SaveHTML renders the final HTML report to the provided output path.
func SaveHTML(content, outputPath string) error {
	tplData, err := os.ReadFile("template.html")
	if err != nil {
		return err
	}

	tpl, err := template.New("output").Parse(string(tplData))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, map[string]any{"Content": content}); err != nil {
		return err
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
