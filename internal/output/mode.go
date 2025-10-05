package output

// Mode defines how results are presented.
type Mode uint8

const (
	// ModeCLI prints endpoints to stdout.
	ModeCLI Mode = 1 << iota
	// ModeHTML renders endpoints into an HTML report.
	ModeHTML
)

// Includes reports whether the provided mode is enabled.
func (m Mode) Includes(target Mode) bool {
	return m&target != 0
}
