package template

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// SetupColorProfile configures lipgloss for output to stderr. When noColor
// is true the Ascii profile is forced; otherwise the best profile detected
// for the terminal is used (or none if stderr isn't a TTY).
func SetupColorProfile(noColor bool) {
	if noColor {
		lipgloss.SetColorProfile(termenv.Ascii)
		return
	}
	if term.IsTerminal(int(os.Stderr.Fd())) {
		lipgloss.SetColorProfile(termenv.NewOutput(os.Stderr).Profile)
	}
}
