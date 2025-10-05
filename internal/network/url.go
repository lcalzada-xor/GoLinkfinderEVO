package network

import "strings"

// CheckURL validates a JS endpoint and resolves it to an absolute URL based on the provided base.
func CheckURL(raw, base string) (string, bool) {
	if !strings.HasSuffix(raw, ".js") {
		return "", false
	}

	parts := strings.Split(raw, "/")
	for _, p := range parts {
		if p == "node_modules" || p == "jquery.js" {
			return "", false
		}
	}

	resolved := raw
	if strings.HasPrefix(resolved, "//") {
		resolved = "https:" + resolved
	} else if !strings.HasPrefix(resolved, "http") {
		if strings.HasPrefix(resolved, "/") {
			resolved = base + resolved
		} else {
			resolved = base + "/" + resolved
		}
	}

	return resolved, true
}
