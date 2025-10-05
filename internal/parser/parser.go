package parser

import (
	"html"
	"regexp"
	"sort"
	"strings"

	jsbeautifier "github.com/ditashi/jsbeautifier-go/jsbeautifier"
	"github.com/ditashi/jsbeautifier-go/optargs"

	"github.com/example/GoLinkfinderEVO/internal/model"
)

const endpointBody = `

  (
    ((?:[a-zA-Z]{1,10}://|//)
    (?:[^"'/]{1,}\.[a-zA-Z]{2,}|[a-zA-Z0-9_\-]{1,})
    [^"']{0,})

    |

    ((?:/|\.\./|\./)
    [^"'><,;| *()(%%$^/\\\\\[\]]
    [^"'><,;|()]{1,})

    |

    ([a-zA-Z0-9_\-/]{1,}/
    [a-zA-Z0-9_\-/.]{1,}
    \.(?:[a-zA-Z]{1,4}|action)
    (?:[\?|#][^"|']{0,}|))

    |

    ([a-zA-Z0-9_\-/]{1,}/
    [a-zA-Z0-9_\-/]{3,}
    (?:[\?|#][^"|']{0,}|))

    |

    ([a-zA-Z0-9_\-]{1,}
    \.(?:php|asp|aspx|jsp|json|
         action|html|js|txt|xml)
    (?:[\?|#][^"|']{0,}|))

  )

`

const rawRegex = "" +
	`
  (?:
    "` + endpointBody + `"
    |
    '` + endpointBody + `'
    |
    ` + "`" + endpointBody + "`" + `
  )

`
const contextDelimiter = "\n"

var endpointRegex = regexp.MustCompile(compactPattern(rawRegex))

var scriptExtensionRegex = mustCompileScriptExtensions([]string{".js", ".mjs", ".jsx", ".ts", ".tsx"})

func compactPattern(pattern string) string {
	var builder strings.Builder
	for _, line := range strings.Split(pattern, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		builder.WriteString(trimmed)
	}
	return builder.String()
}

// EndpointRegex returns the compiled regex used to detect endpoints.
func EndpointRegex() *regexp.Regexp {
	return endpointRegex
}

// ScriptExtensionRegex returns the compiled regex used to validate script file extensions.
func ScriptExtensionRegex() *regexp.Regexp {
	return scriptExtensionRegex
}

// SetScriptExtensions configures the extensions considered as scripts.
// Extensions can be provided with or without a leading dot and are matched case-insensitively.
func SetScriptExtensions(exts []string) {
	scriptExtensionRegex = mustCompileScriptExtensions(exts)
}

// FindEndpoints extracts endpoints from the provided content.
func FindEndpoints(content string, regex *regexp.Regexp, includeContext bool, filter *regexp.Regexp, noDup bool) []model.Endpoint {
	processed := content
	if includeContext {
		processed = beautify(content)
	}

	matches := regex.FindAllStringSubmatchIndex(processed, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	var results []model.Endpoint

	for _, idx := range matches {
		if len(idx) < 4 {
			continue
		}

		linkStart, linkEnd := -1, -1
		for i := 2; i+1 < len(idx); i += 2 {
			if idx[i] >= 0 && idx[i+1] >= 0 {
				linkStart, linkEnd = idx[i], idx[i+1]
				break
			}
		}
		if linkStart == -1 || linkEnd == -1 {
			continue
		}
		matchStart, matchEnd := idx[0], idx[1]
		if linkStart < 0 || linkEnd < 0 || linkStart > len(processed) || linkEnd > len(processed) {
			continue
		}
		link := processed[linkStart:linkEnd]
		if filter != nil && !filter.MatchString(link) {
			continue
		}
		if noDup {
			if _, ok := seen[link]; ok {
				continue
			}
			seen[link] = struct{}{}
		}

		ep := model.Endpoint{Link: link}
		ep.Line = lineNumber(processed, matchStart)
		if includeContext {
			ep.Context = extractContext(processed, matchStart, matchEnd, false)
		}
		results = append(results, ep)
	}

	return results
}

func lineNumber(content string, index int) int {
	if index < 0 {
		return 0
	}

	if index > len(content) {
		index = len(content)
	}

	return strings.Count(content[:index], "\n") + 1
}

func extractContext(content string, matchStart, matchEnd int, includeDelimiter bool) string {
	if matchStart < 0 || matchEnd < 0 {
		return ""
	}

	startSection := content[:matchStart]
	endSection := content[matchEnd:]

	prevIdx := strings.LastIndex(startSection, contextDelimiter)
	nextIdx := strings.Index(endSection, contextDelimiter)

	start := 0
	if prevIdx != -1 {
		start = prevIdx
		if !includeDelimiter {
			start += len(contextDelimiter)
		}
	}

	end := len(content)
	if nextIdx != -1 {
		end = matchEnd + nextIdx
		if includeDelimiter {
			end += len(contextDelimiter)
		}
	}

	if start > len(content) {
		start = len(content)
	}
	if end > len(content) {
		end = len(content)
	}
	if start > end {
		start, end = end, start
	}

	return content[start:end]
}

func beautify(content string) string {
	if len(content) > 1_000_000 {
		replacer := strings.NewReplacer(";", ";\r\n", ",", ",\r\n")
		return replacer.Replace(content)
	}

	options := optargs.MapType{}
	options.Copy(optargs.MapType(jsbeautifier.DefaultOptions()))
	result, err := jsbeautifier.Beautify(&content, options)
	if err != nil {
		return content
	}
	return result
}

// HighlightContext returns the context with the endpoint highlighted for HTML output.
func HighlightContext(context, link string) string {
	escapedContext := html.EscapeString(context)
	escapedLink := html.EscapeString(link)
	return strings.ReplaceAll(escapedContext, escapedLink, "<mark class='highlight'>"+escapedLink+"</mark>")
}

func mustCompileScriptExtensions(exts []string) *regexp.Regexp {
	if len(exts) == 0 {
		return regexp.MustCompile(`(?i)\.(?:js)$`)
	}

	normalized := make([]string, 0, len(exts))
	for _, ext := range exts {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		normalized = append(normalized, strings.ToLower(ext))
	}

	if len(normalized) == 0 {
		return regexp.MustCompile(`(?i)\.(?:js)$`)
	}

	sort.Strings(normalized)

	builder := strings.Builder{}
	builder.WriteString(`(?i)(?:`)
	for idx, ext := range normalized {
		if idx > 0 {
			builder.WriteString(`|`)
		}
		builder.WriteString(regexp.QuoteMeta(ext))
	}
	builder.WriteString(`)$`)

	return regexp.MustCompile(builder.String())
}
