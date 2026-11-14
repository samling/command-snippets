package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
)

func TestThemeValidationRecovery(t *testing.T) {
	for _, tc := range []struct {
		settings string
		valid    bool
	}{
		{"theme: catppuccin-mocha, theme_colors: {focus: '#123456'}", true},
		{"theme: default", true}, {"theme: unknown", false},
		{"theme: catppuccin-mocha, theme_colors: {background: transparent, panel: transparent}", true},
		{"theme_colors: {focus: transparent}", false},
		{"theme_colors: {unknown: '#123456'}", false},
		{"theme_colors: {focus: red}", false},
		{"theme_colors: {focus: '#12345g'}", false},
	} {
		t.Run(tc.settings, func(t *testing.T) {
			lib := fixtureLibrary(t, "settings: {color: always, "+tc.settings+"}\nsnippets:\n  - name: Test\n    command: echo\n")
			if (lib.Error() == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, lib.Error())
			}
		})
	}
}

func TestWorkspaceThemePropagationAndIsolation(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lib := fixtureLibrary(t, "settings: {color: always, theme: catppuccin-mocha, theme_colors: {focus: '#123456'}}\nsnippets:\n  - name: Test\n    command: 'echo {{value}}'\n    inputs: [{name: value, required: true, validate: {pattern: '^[0-9]+$'}}]\n")
	m, err := New(lib, Options{})
	if err != nil || lib.Error() != nil {
		t.Fatalf("%v %v", err, lib.Error())
	}
	original := m.View()
	if !strings.Contains(original, "38;2;18;52;86") || !strings.Contains(original, "48;2;30;30;46") {
		t.Fatalf("theme not applied: %q", original)
	}
	other, _ := New(fixtureLibrary(t, "settings: {color: never}\nsnippets: []\n"), Options{})
	_ = other.View()
	if m.View() != original {
		t.Fatal("independent model changed palette/profile")
	}
	press(m, tea.KeyEnter)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")}) // a real validation error
	if m.Mode != "inputs" || !strings.Contains(m.View(), "38;2;18;52;86") || !strings.Contains(m.View(), "38;2;137;179;250") || !strings.Contains(m.View(), "38;2;243;139;168") {
		t.Fatalf("input focus/preview/error theme missing: %q", m.View())
	}
	press(m, tea.KeyEscape)
	press(m, tea.KeyF2)
	if m.Mode != "editor" || !strings.Contains(m.View(), "38;2;18;52;86") {
		t.Fatal("F2/editor theme missing")
	}
	press(m, tea.KeyEscape)
	press(m, tea.KeyCtrlO)
	if !strings.Contains(m.View(), "catppuccin-mocha") {
		t.Fatal("theme selector missing")
	}
	settings, err := m.Settings.Draft()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(settings.ThemeColors, map[string]string{"focus": "#123456"}) {
		t.Fatal("custom colors lost in settings draft")
	}
	press(m, tea.KeyCtrlS)
	if !strings.Contains(m.View(), "38;2;18;52;86") {
		t.Fatal("saved/reloaded theme lost")
	}
}

func TestInlinePlaceholderInferenceAndWhitespaceTags(t *testing.T) {
	lib := fixtureLibrary(t, "snippets: []\n")
	m, _ := New(lib, Options{Start: "add", NoColor: true})
	e := m.Editor
	e.Name.Set("List")
	e.Build()
	for i, c := range e.Controls {
		if c.Text == &e.Command {
			e.Focus = i
		}
	}
	for _, r := range "ls -lah {{folder_name}} {{folder_name}} {{second}}" {
		if r == ' ' {
			press(m, tea.KeySpace)
		} else {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	if e.Command.String() != "ls -lah {{folder_name}} {{folder_name}} {{second}}" {
		t.Fatalf("physical spaces lost: %q", e.Command.String())
	}
	if len(e.Inputs) != 2 || e.Inputs[0].Name.String() != "folder_name" || !e.Inputs[0].Required {
		t.Fatalf("inference: %+v", e.Inputs)
	}
	if e.Controls[e.Focus].Text != &e.Command {
		t.Fatal("discovery changed command focus")
	}
	if !strings.Contains(e.View(120, 30), "folder_name  text · required") || !strings.Contains(e.View(120, 30), "Command") {
		t.Fatal("input details not inline")
	}
	foundTags := false
	for _, c := range e.Controls {
		if c.Label == "Tags" {
			c.Text.Set("kubernetes\u2003networking Kubernetes networking")
			foundTags = true
		}
		if c.Label == "Destination" {
			t.Fatal("destination remains editable")
		}
	}
	if !foundTags {
		t.Fatal("single Tags control missing")
	}
	e.Build()
	draft, err := e.Draft()
	if err != nil || !reflect.DeepEqual(draft.Tags, []string{"kubernetes", "networking"}) {
		t.Fatalf("tags %v %v", draft.Tags, err)
	}
	first := e.Inputs[0]
	first.Help.Set("Keep me")
	e.Command.Set("ls {{second}} {{port_arg}} {{bad-name}} {{unfinished")
	e.Expressions = append(e.Expressions, pairDraft{text("port_arg"), text("inputs.second")})
	e.Build()
	if len(e.Inputs) != 2 || e.Inputs[0] != first || first.Help.String() != "Keep me" {
		t.Fatal("temporary edits/expressions destroyed or duplicated inputs")
	}
}

func TestEditorAutomaticSourceAndUntouchedMultiwordTags(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	source := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(main, []byte("settings: {sources: ['*.yaml'], default_source: custom.yaml, project_source: false}\nsnippets: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, _ := New(library.Load(main, dir), Options{Start: "add", NoColor: true})
	m.Editor.Name.Set("New")
	m.Editor.Command.Set("echo {{value}}")
	m.Editor.Build()
	press(m, tea.KeyCtrlS)
	if m.Mode != "library" || len(m.Library.Entries) != 1 || m.Library.Entries[0].Source.Path != source || m.Command != "" {
		t.Fatalf("source/inference save: %s %s", m.Mode, m.Message)
	}
	entry := m.Library.Entries[0]
	entry.Snippet.Tags = []string{"two words", "networking"}
	e, err := NewEditor(entry, "wrong.yaml")
	if err != nil {
		t.Fatal(err)
	}
	draft, _ := e.Draft()
	if !reflect.DeepEqual(draft.Tags, entry.Snippet.Tags) || e.Dirty() {
		t.Fatal("untouched legacy tags changed")
	}
	if e.Destination.String() != source {
		t.Fatal("existing source lost")
	}
	m.Editor = e
	m.Mode = "editor"
	e.Command.Set("echo {{bad-name}}")
	before, _ := os.ReadFile(source)
	press(m, tea.KeyCtrlS)
	after, _ := os.ReadFile(source)
	if string(before) != string(after) || m.Mode != "editor" || e.Error == "" {
		t.Fatal("failed save changed source")
	}
}

func TestThemedEditorFocusGeometryAndNoColor(t *testing.T) {
	for _, plain := range []bool{false, true} {
		t.Setenv("NO_COLOR", "")
		lib := fixtureLibrary(t, "settings: {color: always, theme: catppuccin-mocha}\nsnippets: []\n")
		m, _ := New(lib, Options{Start: "add", NoColor: plain})
		e := m.Editor
		e.Name.Set(strings.Repeat("Long title 日本語 ", 20))
		e.Command.Set("ls -lah {{folder_name}}")
		e.Build()
		for _, size := range [][2]int{{120, 30}, {90, 24}, {70, 16}, {40, 16}, {40, 10}, {24, 6}} {
			m.Width, m.Height = size[0], size[1]
			for i := range e.Controls {
				e.Focus = i
				view := m.View()
				assertReviewDimensions(t, view, size)
				if !strings.Contains(view, "╭") || !strings.Contains(view, "Esc:back") || !strings.Contains(view, "Saved to:") {
					t.Fatalf("frame/source/footer absent at %v focus=%d: %s", size, i, view)
				}
				if plain && strings.Contains(view, "\x1b") {
					t.Fatal("themed no-color editor emitted ANSI")
				}
			}
		}
	}
	t.Setenv("NO_COLOR", "1")
	m, _ := New(fixtureLibrary(t, "settings: {color: always, theme: catppuccin-mocha}\nsnippets: []\n"), Options{Start: "add"})
	if strings.Contains(m.View(), "\x1b") || !strings.Contains(m.View(), "|") {
		t.Fatal("NO_COLOR must override always and keep a textual cursor")
	}
}

func TestInlineTestActionReturnsToCommandWithoutEmission(t *testing.T) {
	m, _ := New(fixtureLibrary(t, "snippets: []\n"), Options{Start: "add", NoColor: true})
	e := m.Editor
	e.Name.Set("List")
	e.Command.Set("ls -lah {{folder_name}}")
	e.Build()
	for i, c := range e.Controls {
		if c.Label == "Test preview" {
			e.Focus = i
		}
	}
	press(m, tea.KeyEnter)
	if e.Test == nil || e.Section != 3 {
		t.Fatal("inline Test action missing")
	}
	if e.Controls[e.Focus].Text != e.Test.Texts["folder_name"] {
		t.Fatal("Test preview did not focus the inline test value")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("two words")})
	press(m, tea.KeyEnter)
	if !e.Test.Result.Valid() || e.Test.Result.Command != "ls -lah 'two words'" || m.Command != "" || m.Done {
		t.Fatal("preview execution/emission or quoting changed")
	}
	press(m, tea.KeyTab)
	if e.Controls[e.Focus].Text != &e.Name {
		t.Fatal("Tab did not return to command pane")
	}
}

func TestSettingsSurfaceTransparencySaveReload(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lib := fixtureLibrary(t, "settings: {color: always, theme: catppuccin-mocha, theme_colors: {background: '#112233', panel: '#445566', focus: '#123456'}}\nsnippets: []\n")
	m, err := New(lib, Options{})
	if err != nil {
		t.Fatal(err)
	}
	press(m, tea.KeyCtrlO)
	for _, label := range []string{"Transparent canvas", "Transparent panels"} {
		for i, c := range m.Settings.Controls {
			if c.Label == label {
				m.Settings.Focus = i
				press(m, tea.KeyRight)
				break
			}
		}
	}
	press(m, tea.KeyCtrlS)
	if m.Library.Error() != nil || m.Mode != "library" || !reflect.DeepEqual(m.Library.Settings.ThemeColors, map[string]string{"background": "transparent", "panel": "transparent", "focus": "#123456"}) {
		t.Fatalf("saved transparency lost: mode=%s colors=%v error=%v", m.Mode, m.Library.Settings.ThemeColors, m.Library.Error())
	}
	if strings.Contains(m.View(), "48;2;30;30;46") || strings.Contains(m.View(), "48;2;24;24;37") || !strings.Contains(m.View(), "38;2;18;52;86") {
		t.Fatal("saved transparency did not reload styles")
	}
	before, err := os.ReadFile(lib.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	press(m, tea.KeyCtrlO)
	press(m, tea.KeyCtrlS)
	after, err := os.ReadFile(lib.ConfigPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("no-op save changed transparent config")
	}
}

func TestSettingsSurfaceTransparencyPreservesOverrides(t *testing.T) {
	for _, overrides := range []map[string]string{
		{"background": "#112233", "panel": "#445566", "focus": "#123456"},
		{"background": "transparent", "panel": "transparent", "focus": "#123456"},
		{"focus": "#123456"},
	} {
		e := NewSettingsEditor(&library.Library{Settings: models.Settings{Color: "auto", Theme: "catppuccin-mocha", ThemeColors: overrides}})
		draft, err := e.Draft()
		if err != nil || e.Dirty() || !reflect.DeepEqual(draft.ThemeColors, overrides) {
			t.Fatalf("no-op changed overrides: %v %v", draft, err)
		}
		for _, label := range []string{"Transparent canvas", "Transparent panels"} {
			found := false
			for i, c := range e.Controls {
				if c.Label == label {
					found = true
					e.Focus = i
					e.Update(tea.KeyMsg{Type: tea.KeyRight})
					draft, err = e.Draft()
					if err != nil || draft.ThemeColors["focus"] != "#123456" {
						t.Fatalf("toggle damaged unrelated override: %v %v", draft, err)
					}
					role := "background"
					if label == "Transparent panels" {
						role = "panel"
					}
					if (draft.ThemeColors[role] == "transparent") == (overrides[role] == "transparent") {
						t.Fatalf("toggle did not change %s", role)
					}
					e.Update(tea.KeyMsg{Type: tea.KeyLeft})
					draft, _ = e.Draft()
					if !reflect.DeepEqual(draft.ThemeColors, overrides) || e.Dirty() {
						t.Fatal("round trip lost custom opaque override")
					}
				}
			}
			if !found {
				t.Fatalf("Settings missing %s", label)
			}
		}
	}
}
