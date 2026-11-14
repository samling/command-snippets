package workspace

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func openInlineTest(t *testing.T, m *Model) {
	t.Helper()
	for i, c := range m.Editor.Controls {
		if c.Label == "Test preview" {
			m.Editor.Focus = i
			press(m, tea.KeyEnter)
			if m.Editor.Test == nil {
				t.Fatal("Test action failed")
			}
			return
		}
	}
	t.Fatal("Test action missing")
}

func TestInlineTestRepeatActionsKeepEditorFocus(t *testing.T) {
	lib := fixtureLibrary(t, `settings: {project_source: false}
snippets:
  - name: Repeat test
    command: echo {{items}} {{b}} {{c}}
    inputs:
      - {name: items, kind: repeat, flag: -i}
      - {name: b, default: B}
      - {name: c, default: C}
`)
	m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0], NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	openInlineTest(t, m)
	e := m.Editor
	e.Focus = e.TestStart
	for n := 0; n < 2; n++ {
		press(m, tea.KeyEnter)
		if got := e.Controls[e.Focus].ID; got != "items" {
			t.Fatalf("repeat Add moved focus to %q, Form ID=%q", got, e.Test.Controls[e.Test.Focus].ID)
		}
	}
	for i, c := range e.Controls {
		if c.ID == "items-remove-1" {
			e.Focus = i
		}
	}
	press(m, tea.KeyEnter)
	if e.Controls[e.Focus].ID != e.Test.Controls[e.Test.Focus].ID {
		t.Fatal("repeat removal desynchronized editor and form focus")
	}
	if m.Command != "" || m.Done {
		t.Fatal("Test emitted executable command")
	}
}

func TestCompactInlineTestKeepsPreviewAndActions(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for _, plain := range []bool{false, true} {
		for _, size := range [][2]int{{120, 30}, {90, 24}, {60, 18}, {40, 10}, {24, 6}} {
			t.Run(fmt.Sprintf("plain=%v/%dx%d", plain, size[0], size[1]), func(t *testing.T) {
				lib := fixtureLibrary(t, `settings: {project_source: false, color: always, theme: catppuccin-mocha}
snippets:
  - name: Compact preview
    command: echo {{value}}
    inputs: [{name: value, default: UNIQUE}]
`)
				m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0], NoColor: plain})
				if err != nil {
					t.Fatal(err)
				}
				openInlineTest(t, m)
				m.Width, m.Height = size[0], size[1]
				view := m.View()
				if !strings.Contains(view, "Preview:") || !strings.Contains(view, "echo") || !strings.Contains(view, "UNIQUE") || !strings.Contains(view, "Ctrl-S:save Esc:back") {
					t.Fatalf("compact Test loses preview/actions:\n%s", view)
				}
				assertReviewDimensions(t, view, size)
				if plain && strings.Contains(view, "\x1b") {
					t.Fatal("no-color preview emitted ANSI")
				}
			})
		}
	}
}

func TestDefaultTextOnlyOverrideReachesHelpRowsAndValues(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lib := fixtureLibrary(t, `settings: {color: always, theme: default, theme_colors: {text: '#123456'}}
snippets:
  - name: Unique row
    command: echo {{value}}
    inputs: [{name: value, default: UNIQUE}]
`)
	m, err := New(lib, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("help", func(t *testing.T) {
		m.Help = true
		defer func() { m.Help = false }()
		if !strings.Contains(m.View(), "38;2;18;52;86") {
			t.Fatal("text-only override absent from help without background override")
		}
	})
	t.Run("active-row", func(t *testing.T) {
		m.Focus = 2
		if !strings.Contains(m.results(40, 20), "38;2;18;52;86") {
			t.Fatal("active row ignores explicit text override")
		}
	})
	t.Run("input-value", func(t *testing.T) {
		press(m, tea.KeyEnter)
		if m.Form == nil {
			t.Fatal("no input form")
		}
		for _, line := range strings.Split(m.View(), "\n") {
			if strings.Contains(line, "UNIQUE") && strings.Contains(line, "38;2;18;52;86") {
				return
			}
		}
		t.Fatal("typed input value ignores text-only override")
	})
}

func TestSettingsFailureUsesThemeErrorRole(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for _, plain := range []bool{false, true} {
		t.Run(fmt.Sprintf("plain=%v", plain), func(t *testing.T) {
			lib := fixtureLibrary(t, `settings: {color: always, theme: catppuccin-mocha, theme_colors: {error: '#123456'}}
snippets: []
`)
			m, err := New(lib, Options{NoColor: plain})
			if err != nil {
				t.Fatal(err)
			}
			press(m, tea.KeyCtrlO)
			m.Settings.Sources = append(m.Settings.Sources, text("["))
			m.Settings.Build()
			press(m, tea.KeyCtrlS)
			if m.Mode != "settings" || m.Settings.Error == "" {
				t.Fatal("invalid source did not produce Settings failure")
			}
			view := m.View()
			if plain {
				if strings.Contains(view, "\x1b") {
					t.Fatal("no-color error has ANSI")
				}
			} else if !strings.Contains(view, "38;2;18;52;86") {
				t.Fatal("Settings failure ignores error theme role")
			}
		})
	}
}

func TestInlineTestRetainsIndependentExpressionPreview(t *testing.T) {
	lib := fixtureLibrary(t, `settings: {project_source: false}
snippets:
  - name: Docker preview
    command: docker run {{port_arg}} {{image_arg}}
    inputs:
      - {name: port}
      - {name: image, required: true}
    expressions:
      port_arg: 'inputs.port == "" ? "" : flag("-p", inputs.port + ":" + inputs.port)'
      image_arg: inputs.image
`)
	m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0], NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	openInlineTest(t, m)
	m.Editor.Focus = m.Editor.TestStart
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8080")})
	view := m.View()
	if !strings.Contains(view, "-p 8080:8080") || !strings.Contains(view, "{{image_arg}}") {
		t.Fatalf("inline Test loses partial expression preview:\n%s", view)
	}
	if m.Editor.Test.Result.Valid() || m.Command != "" || m.Done {
		t.Fatal("partial preview became executable")
	}
}
