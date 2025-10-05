package output

import _ "embed"

// templateHTML contains the embedded HTML report template.
//
//go:embed template.html
var templateHTML string
