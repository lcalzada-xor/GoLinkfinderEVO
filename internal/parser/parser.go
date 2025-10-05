package parser

import (
	"html"
	"regexp"
	"strings"

	jsbeautifier "github.com/ditashi/jsbeautifier-go/jsbeautifier"
	"github.com/ditashi/jsbeautifier-go/optargs"

	"github.com/example/GoLinkfinderEVO/internal/model"
)

const rawRegex = `

  (?:"|')

  (
    ((?:[a-zA-Z]{1,10}://|//)
    [^"'/]{1,}\.
    [a-zA-Z]{2,}[^"']{0,})

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

  (?:"|')

`
const contextDelimiter = "\n"

var endpointRegex = regexp.MustCompile(compactPattern(rawRegex))

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
		linkStart, linkEnd := idx[2], idx[3]
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
		if includeContext {
			ep.Context = extractContext(processed, matchStart, matchEnd, false)
		}
		results = append(results, ep)
	}

	return results
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
	highlight := strings.ReplaceAll(escapedContext, escapedLink, "<span style='background-color:yellow'>"+escapedLink+"</span>")
	return highlight
}
