package template

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/samling/command-snippets/internal/models"
)

func TestLongDiagnosticHeaderKeepsFormActionsVisible(t *testing.T) {
	profile := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(profile)
	SetupColorProfile(true)
	f, err := NewForm(models.Snippet{
		Name:        strings.Repeat("n", 240),
		Command:     "echo {{bad}}",
		Inputs:      []models.Input{{Name: "x"}},
		Expressions: map[string]string{"bad": `"\x00"`},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{40, 12}, {24, 12}, {120, 12}, {40, 10}, {24, 6}} {
		view := f.View(size[0], size[1])
		if !strings.Contains(view, "Enter:submit") || !strings.Contains(view, "Esc:back Ctrl-C:cancel") {
			t.Fatalf("long diagnostic header hides actions at %v: %s", size, view)
		}
		if !strings.Contains(view, "NUL") || f.Result.Command != "" || f.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
			t.Fatal("diagnostic hidden or invalid command emitted")
		}
		if strings.Count(view, "\n")+1 > size[1] {
			t.Fatalf("height exceeded %v: %s", size, view)
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("width exceeded %v: %s", size, line)
			}
		}
	}
}
