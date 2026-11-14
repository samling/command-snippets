package template

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/samling/command-snippets/internal/models"
)

func TestNonfieldDiagnosticsVisibleAndFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		expressions   map[string]string
		want          string
	}{
		{"limit", "echo {{x}}", nil, "rendered command exceeds 1 MiB"},
		{"NUL", "echo {{bad}}", map[string]string{"bad": `"\x00"`}, "NUL cannot appear in a shell command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := NewForm(models.Snippet{Name: "Example", Command: tc.command, Inputs: []models.Input{{Name: "x"}}, Expressions: tc.expressions}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "limit" {
				f.Texts["x"].Set(strings.Repeat("x", 1<<20))
				f.Refresh()
			}
			if f.Result.Valid() || f.Result.Command != "" || f.Update(tea.KeyMsg{Type: tea.KeyCtrlS}) {
				t.Fatal("invalid command submitted")
			}
			for _, size := range [][2]int{{120, 30}, {40, 10}, {24, 6}} {
				view := f.View(size[0], size[1])
				if !strings.Contains(strings.Join(strings.Fields(view), " "), tc.want) {
					t.Fatalf("diagnostic hidden at %v: %s", size, view)
				}
				if !strings.Contains(view, "Esc:back Ctrl-C:cancel") {
					t.Fatalf("cancel footer hidden: %s", view)
				}
				if strings.Count(view, "\n")+1 > size[1] {
					t.Fatalf("height exceeded: %s", view)
				}
				for _, line := range strings.Split(view, "\n") {
					if ansi.StringWidth(line) > size[0] {
						t.Fatalf("width exceeded: %s", line)
					}
				}
			}
		})
	}
	f, err := NewForm(models.Snippet{Name: "Hidden", Command: "echo {{x}}", Inputs: []models.Input{{Name: "x"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Result.Errors = map[string]string{"z": "last\x1b[31m", "a": "hidden diagnostic", "x": "visible field diagnostic"}
	f.Controls[0].Error = f.Result.Errors["x"]
	f.Result.Visible["a"] = false
	view := f.View(120, 30)
	if !strings.Contains(view, "a: hidden diagnostic") || !strings.Contains(view, `z: last\x1b[31m`) || strings.Contains(view, "\x1b") || strings.Index(view, "a: hidden") > strings.Index(view, "z: last") {
		t.Fatalf("missing/unsafe/unordered diagnostics: %s", view)
	}
	if strings.Count(view, "visible field diagnostic") != 1 {
		t.Fatalf("field diagnostic duplicated: %s", view)
	}
}

func TestEnterNavigatesInputsAndSubmitsLastFieldWhenValid(t *testing.T) {
	SetupColorProfile(true)
	s := models.Snippet{Name: "Two fields", Command: "echo {{first}} {{second}}", Inputs: []models.Input{{Name: "first", Default: "one"}, {Name: "second", Required: true}}}
	f, err := NewForm(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("incomplete form submitted")
	}
	f.Texts["second"].Set("two")
	f.Refresh()
	f.Focus = 0
	if f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || f.Focus != 1 {
		t.Fatal("Enter did not advance to the last field")
	}
	if !f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || f.Result.Command != "echo one two" {
		t.Fatal("Enter did not submit the completed form")
	}
	if !f.Update(tea.KeyMsg{Type: tea.KeyCtrlS}) {
		t.Fatal("optional Ctrl-S alias stopped working")
	}
	for _, size := range [][2]int{{90, 24}, {40, 10}, {24, 6}} {
		view := f.View(size[0], size[1])
		if !strings.Contains(view, "Esc:back") || !strings.Contains(view, "Enter:submit") || strings.Contains(view, "Ctrl-S:use") {
			t.Fatalf("primary Enter controls clipped or replaced at %v: %q", size, view)
		}
	}
}

func TestInputFormArrowNavigationAndOriginalFieldDescriptions(t *testing.T) {
	SetupColorProfile(true)
	f, err := NewForm(models.Snippet{Name: "Example", Command: "echo {{first}} {{second}}", Inputs: []models.Input{{Name: "first", Help: "First value"}, {Name: "second", Help: "Second value"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Update(tea.KeyMsg{Type: tea.KeyDown})
	if f.Focus != 1 {
		t.Fatal("Down did not move to next input")
	}
	f.Update(tea.KeyMsg{Type: tea.KeyUp})
	if f.Focus != 0 {
		t.Fatal("Up did not move to previous input")
	}
	view := ansi.Strip(f.View(120, 30))
	if !strings.Contains(view, "Live preview") || !strings.Contains(view, "First value") || strings.Contains(view, "Second value") || strings.Contains(view, "first (First value)") || strings.Contains(view, "second (Second value)") {
		t.Fatalf("original preview/description layout lost: %s", view)
	}
}
