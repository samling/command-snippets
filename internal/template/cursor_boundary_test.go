package template

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestWideBlockCursorRemainsVisibleWhenClipped(t *testing.T) {
	profile := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(profile)
	lipgloss.SetColorProfile(termenv.TrueColor)
	text := NewText(strings.Repeat("a", 13)+"日", false)
	text.Cursor = 13
	view := text.OneLine(14, true)
	if !strings.Contains(view, "\x1b[7m日") || ansi.StringWidth(view) > 14 {
		t.Fatalf("wide cursor clipped: %q", view)
	}
	view = text.OneLine(1, true)
	if !strings.Contains(view, "\x1b[7m") || ansi.StringWidth(view) != 1 {
		t.Fatalf("single-cell cursor fallback missing: %q", view)
	}
	other := NewText("other", false)
	view = ControlsView([]Control{{Label: "X", Text: &text}, {Label: "Other", Text: &other}}, 0, 19, 1)
	if !strings.Contains(view, "\x1b[7m日") || ansi.StringWidth(view) > 19 {
		t.Fatalf("wrapped focused cursor scrolled out: %q", view)
	}
}
