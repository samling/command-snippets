package workspace

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/templating"
)

func TestStructuralActionsInvalidateEditorTest(t *testing.T) {
	for _, tc := range []struct {
		section     int
		action      string
		wantInvalid bool
	}{
		{1, "Remove this input", true}, {1, "Remove Choice", false}, {2, "Remove Expression", true},
	} {
		t.Run(tc.action, func(t *testing.T) {
			lib := fixtureLibrary(t, "snippets:\n  - name: Example\n    command: echo {{x}} {{extra}}\n    inputs:\n      - name: x\n        kind: choice\n        choices: [{label: One, value: one}, {label: Two, value: two}]\n    expressions: {extra: '\"extra\"'}\n")
			m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0], NoColor: true})
			if err != nil {
				t.Fatal(err)
			}
			m.Width, m.Height = 90, 24
			press(m, tea.KeyCtrlLeft)
			if m.Editor.Test == nil || !m.Editor.Test.Result.Valid() || m.Editor.Test.Result.Command != "echo one extra" {
				t.Fatal("initial Test not valid")
			}
			*m.Editor.Test.Choices["x"] = "Two"
			m.Editor.Test.Refresh()
			for m.Editor.Section != tc.section {
				press(m, tea.KeyCtrlLeft)
			}
			found := false
			for i, c := range m.Editor.Controls {
				if strings.HasPrefix(c.Label, tc.action) && c.Action != nil {
					m.Editor.Focus = i
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("action absent: %s, controls=%+v", tc.action, m.Editor.Controls)
			}
			m.Editor.Error = "stale error"
			press(m, tea.KeyEnter)
			if m.Editor.Test != nil || m.Editor.Error != "" {
				t.Fatalf("cached test/error survived action: %+v", m.Editor)
			}
			draft, err := m.Editor.Draft()
			if err != nil {
				t.Fatal(err)
			}
			_, compileErr := templating.Compile(draft)
			if (compileErr != nil) != tc.wantInvalid {
				t.Fatalf("draft validity: %v", compileErr)
			}
			for m.Editor.Section != 3 {
				press(m, tea.KeyCtrlRight)
			}
			if tc.wantInvalid {
				if m.Editor.Test != nil && m.Editor.Test.Result.Valid() {
					t.Fatal("stale valid Test")
				}
				if m.Editor.Error == "" {
					t.Fatal("missing compile diagnostic")
				}
			} else {
				if m.Editor.Test == nil || !m.Editor.Test.Result.Valid() || m.Editor.Test.Result.Command != "echo two extra" {
					t.Fatal("choice Test/default did not refresh")
				}
			}
		})
	}
}

func TestEditorDefaultRowActionPreservesCachedTestValues(t *testing.T) {
	lib := fixtureLibrary(t, "snippets:\n  - name: Repeat\n    command: echo {{x}}\n    inputs:\n      - name: x\n        kind: repeat\n        flag: -e\n        default: [one, two]\n")
	m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0], NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 90, 24
	press(m, tea.KeyCtrlLeft)
	m.Editor.Test.Repeats["x"][0].Set("custom")
	m.Editor.Test.Refresh()
	press(m, tea.KeyCtrlLeft)
	press(m, tea.KeyCtrlLeft)
	found := false
	for i, c := range m.Editor.Controls {
		if c.Label == "Remove Default item" && c.Action != nil {
			m.Editor.Focus = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("default row action absent")
	}
	press(m, tea.KeyEnter)
	if m.Editor.Test != nil {
		t.Fatal("cached Test survived default action")
	}
	draft, err := m.Editor.Draft()
	if err != nil {
		t.Fatal(err)
	}
	defaults, ok := draft.Inputs[0].Default.([]string)
	if !ok || len(defaults) != 1 || defaults[0] != "two" {
		t.Fatalf("default removal failed: %+v", draft)
	}
	press(m, tea.KeyCtrlRight)
	press(m, tea.KeyCtrlRight)
	if m.Editor.Test == nil || !m.Editor.Test.Result.Valid() || m.Editor.Test.Result.Command != "echo -e custom -e two" {
		t.Fatal("current-draft rebuild lost cached Test values")
	}
}

func TestEditorTestDisplaysNonfieldFailure(t *testing.T) {
	for _, tc := range []struct{ name, command, want string }{
		{"limit", "echo {{x}}", "rendered command exceeds 1 MiB"},
		{"NUL", `echo {{bad}}`, "NUL cannot appear in a shell command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "snippets:\n  - name: Example\n    command: '" + tc.command + "'\n    inputs: [{name: x}]\n"
			if tc.name == "NUL" {
				body += `    expressions: {bad: '"\x00"'}` + "\n"
			}
			lib := fixtureLibrary(t, body)
			if err := lib.Error(); err != nil {
				t.Fatal(err)
			}
			e, err := NewEditor(lib.Entries[0], "")
			if err != nil {
				t.Fatal(err)
			}
			e.Section = 3
			e.Build()
			if tc.name == "limit" {
				e.Test.Texts["x"].Set(strings.Repeat("x", 1<<20))
				e.Test.Refresh()
			}
			if e.Test.Result.Valid() || e.Test.Result.Command != "" {
				t.Fatal("invalid Test command usable")
			}
			for _, size := range [][2]int{{120, 30}, {40, 10}, {24, 6}} {
				view := e.View(size[0], size[1])
				if !strings.Contains(strings.Join(strings.Fields(view), " "), tc.want) {
					t.Fatalf("Test diagnostic hidden at %v: %s", size, view)
				}
				if !strings.Contains(view, "Esc:back") || strings.Count(view, "\n")+1 > size[1] {
					t.Fatalf("Test geometry/footer changed: %s", view)
				}
			}
		})
	}
}

func TestHelpCtrlCCancellationAndDirtyConfirmation(t *testing.T) {
	for _, mode := range []string{"library", "editor", "settings"} {
		t.Run(mode, func(t *testing.T) {
			lib := fixtureLibrary(t, "snippets:\n  - name: Example\n    command: echo safe\n")
			m, err := New(lib, Options{NoColor: true})
			if err != nil {
				t.Fatal(err)
			}
			m.Width, m.Height = 90, 24
			switch mode {
			case "editor":
				press(m, tea.KeyCtrlE)
				m.Editor.Name.Set("changed")
			case "settings":
				press(m, tea.KeyCtrlO)
				m.Settings.Color = "never"
			}
			press(m, tea.KeyF1)
			press(m, tea.KeyEsc)
			if m.Help || m.Done || m.ConfirmDiscard {
				t.Fatal("Esc must only dismiss help")
			}
			press(m, tea.KeyF1)
			press(m, tea.KeyCtrlC)
			if mode != "library" {
				if !m.ConfirmDiscard || !m.DiscardCancels || m.Done || m.Command != "" {
					t.Fatal("dirty help cancellation skipped confirmation")
				}
				m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
				if m.Done || m.ConfirmDiscard {
					t.Fatal("declined discard cancelled")
				}
				press(m, tea.KeyF1)
				press(m, tea.KeyCtrlC)
				m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
			}
			if !m.Done || !m.Cancelled || m.Command != "" {
				t.Fatal("help consumed cancellation or emitted command")
			}
		})
	}
}
