package output

// Mode defines how results are presented.
type Mode int

const (
	// ModeCLI prints endpoints to stdout.
	ModeCLI Mode = iota
	// ModeHTML renders endpoints into an HTML report.
	ModeHTML
)
