package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
	"gopkg.in/yaml.v3"
)

func fixtureLibrary(t *testing.T, body string) *library.Library {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return library.Load(path, dir)
}
func press(m *Model, key tea.KeyType) { m.Update(tea.KeyMsg{Type: key}) }
func TestLibraryFocusIdentityResizeAndCancellation(t *testing.T) {
	lib := fixtureLibrary(t, "snippets:\n  - name: Duplicate\n    tags: [Kubernetes, Monitoring]\n    command: echo one\n  - name: Duplicate\n    tags: [Kubernetes]\n    command: echo two\n")
	m, err := New(lib, Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q/jk日本")})
	if m.Query.String() != "q/jk日本" || len(m.Matches) != 0 {
		t.Fatal("query hotkey collision")
	}
	press(m, tea.KeyCtrlL)
	m.Focus = 1
	m.Category = 3
	m.toggleCategory()
	if len(m.Matches) != 1 {
		t.Fatal("tag category mismatch")
	}
	m.Category = 2
	m.toggleCategory()
	if len(m.Matches) != 1 {
		t.Fatal("tag intersection failed")
	}
	m.Category = 0
	m.toggleCategory()
	if len(m.Matches) != 2 {
		t.Fatal("All did not clear")
	}
	for _, size := range [][2]int{{140, 40}, {120, 20}, {90, 24}, {60, 18}, {40, 10}, {20, 5}, {0, 0}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		_ = m.View()
		if len(m.Matches) != 2 {
			t.Fatal("resize changed data")
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	m.Focus = 2
	press(m, tea.KeyDown)
	if m.selected() != lib.Entries[1] {
		t.Fatal("duplicate identity lost")
	}
	press(m, tea.KeyEsc)
	if !m.Cancelled || m.Command != "" {
		t.Fatal("cancel emitted")
	}
}
func TestEditorRoundTripEveryCanonicalFieldAndConflictRetention(t *testing.T) {
	s := models.Snippet{Name: "Friendly", Description: "Description", Tags: []string{"Examples"}, Command: "echo {{text}} {{flag}} {{choice}} {{repeat}} {{toggle}} {{out}}", Inputs: []models.Input{
		{Name: "text", Label: "Text", Help: "Help", Default: "42", Special: map[string]string{"": "empty", "all": "literal"}, Validate: &models.Validation{Pattern: ".*", Range: []int{1, 999}, Regex: true}, VisibleWhen: &models.Condition{Expr: `inputs.toggle`}, RequiredWhen: &models.Condition{Input: "toggle", Equals: true}},
		{Name: "flag", Kind: "flag", Flag: "-n", Default: ""}, {Name: "choice", Kind: "choice", Choices: []models.Choice{{Label: "One", Value: "1"}, {Label: "Two", Value: "2"}}, Default: "One"}, {Name: "repeat", Kind: "repeat", Flag: "-e", Default: []string{"A=1", "B=hello world"}}, {Name: "toggle", Kind: "toggle", Flag: "-x", Default: false},
	}, Expressions: map[string]string{"out": `quote(inputs.text)`}}
	entry := &library.Entry{Snippet: s, Source: &library.Source{DisplayPath: "source.yaml"}}
	editor, err := NewEditor(entry, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := editor.Draft()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := yaml.Marshal(s)
	after, _ := yaml.Marshal(got)
	if string(before) != string(after) {
		t.Fatalf("round trip lost fields:\n%s\n%s", before, after)
	}
	lib := fixtureLibrary(t, "snippets:\n  - name: Friendly\n    command: echo ok\n")
	m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0], NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Editor.Name.Set("Renamed")
	if err := os.WriteFile(lib.ConfigPath, []byte("snippets: []\n# external\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m.saveEditor()
	if m.Mode != "editor" || m.Editor.Error == "" || !m.Editor.Dirty() {
		t.Fatal("conflict discarded draft")
	}
	press(m, tea.KeyEsc)
	if !m.ConfirmDiscard {
		t.Fatal("dirty cancellation did not ask")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.ConfirmDiscard || m.Mode != "editor" {
		t.Fatal("keep editing failed")
	}
}
func TestMarkChangesOnlyExactRangeAndAutomaticInputConfiguration(t *testing.T) {
	e, err := NewEditor(nil, "main.yaml")
	if err != nil {
		t.Fatal(err)
	}
	e.Command.Set("echo value value")
	e.Command.Cursor = 5
	e.Mark()
	e.Command.Cursor = 10
	e.Mark()
	e.PickerName.Set("message")
	e.confirmMark()
	if e.Command.String() != "echo {{message}} value" || len(e.Inputs) != 1 {
		t.Fatal("mark replaced another occurrence")
	}
	e.Section = 0
	e.Command.Set("echo {{undefined}}")
	e.Build()
	if len(e.Inputs) != 2 || e.Inputs[1].Name.String() != "undefined" || !e.Inputs[1].Required {
		t.Fatal("undefined input not inferred")
	}
	listed := false
	for _, control := range e.Controls {
		if control.Label == "undefined" && control.Action != nil {
			listed = true
		}
	}
	if !listed {
		t.Fatal("inferred input missing from the inline input list")
	}
	e.openInput(e.Inputs[1])
	e.Build()
	found := false
	for _, control := range e.Controls {
		if control.Text == &e.Inputs[1].Name {
			found = true
		}
	}
	if !found || e.Section != 0 {
		t.Fatal("inferred input not inline")
	}

}
func TestSourceSettingsRecoveryAndNoCommandOnSave(t *testing.T) {
	lib := fixtureLibrary(t, "settings:\n  sources: [missing.yaml]\nsnippets: []\n")
	m, err := New(lib, Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	if m.Mode != "recovery" {
		t.Fatal("invalid source not recovery")
	}
	press(m, tea.KeyCtrlO)
	if m.Mode != "settings" {
		t.Fatal("settings unavailable")
	}
	m.Settings.Sources = nil
	press(m, tea.KeyCtrlS)
	if m.Mode != "library" || m.Library.Error() != nil || m.Command != "" {
		t.Fatal("recovery save failed")
	}
	press(m, tea.KeyCtrlN)
	m.Editor.Name.Set("New command")
	m.Editor.Command.Set("echo new")
	press(m, tea.KeyCtrlS)
	if m.Mode != "library" || len(m.Library.Entries) != 1 || m.Command != "" || !models.ValidID(m.Library.Entries[0].Snippet.ID) {
		t.Fatalf("save emitted or failed: %+v", m)
	}
}

func TestRecoveryShowsOnlyAvailableSettingsShortcut(t *testing.T) {
	for _, test := range []struct {
		name, body        string
		settingsAvailable bool
	}{
		{"malformed YAML", "settings: [\n", false},
		{"legacy settings", "settings:\n  additional_configs: [snippets/*.yaml]\n", false},
		{"invalid settings", "settings:\n  color: invalid\n", false},
		{"included source error", "settings:\n  sources: [missing.yaml]\nsnippets: []\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, err := New(fixtureLibrary(t, test.body), Options{NoColor: true})
			if err != nil {
				t.Fatal(err)
			}
			if m.Mode != "recovery" {
				t.Fatal("expected recovery mode")
			}
			view := m.View()
			if strings.Contains(view, "if main YAML parses") {
				t.Fatal("footer describes unavailable settings condition")
			}
			if strings.Contains(view, "Ctrl-O: settings") != test.settingsAvailable {
				t.Fatalf("settings shortcut availability mismatch: %s", view)
			}
			if !strings.Contains(view, "Ctrl-R: retry") || !strings.Contains(view, "Esc: cancel") {
				t.Fatal("missing recovery actions")
			}
			press(m, tea.KeyCtrlO)
			if (m.Mode == "settings") != test.settingsAvailable {
				t.Fatalf("settings action availability mismatch: %s", m.Mode)
			}
		})
	}
}

func TestSlashJumpsToSearchFromLibraryPanes(t *testing.T) {
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: One\n    command: echo one\n    tags: [a]\n  - name: Two\n    command: echo two\n    tags: [b]\n")
	m, err := New(lib, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if lib.Error() != nil {
		t.Fatal(lib.Error())
	}
	m.Width, m.Height = 120, 30
	for _, pane := range []int{1, 2, 3} {
		m.Focus = pane
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
		if m.Focus != 0 || m.Query.String() != "" {
			t.Fatalf("pane %d: / focus=%d query=%q", pane, m.Focus, m.Query.String())
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a/b")})
	if m.Query.String() != "a/b" {
		t.Fatalf("/ inside the search box must type a slash: %q", m.Query.String())
	}
}

func TestVersionVisibleInTitleEditorAndHelp(t *testing.T) {
	defer func(old string) { Version = old }(Version)
	Version = "v9.8.7-3-gabc1234-dirty"
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: One\n    command: echo one\n")
	m, err := New(lib, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{120, 30}, {60, 18}} {
		m.Width, m.Height = size[0], size[1]
		first := strings.SplitN(ansi.Strip(m.View()), "\n", 2)[0]
		if !strings.Contains(first, "CS — command library") || !strings.Contains(first, Version) {
			t.Fatalf("%v title line lacks the version: %q", size, first)
		}
		if ansi.StringWidth(first) > size[0] {
			t.Fatalf("%v title line overflows: %q", size, first)
		}
	}
	m.Width, m.Height = 30, 10 // too narrow: the title wins, the version is dropped
	if first := strings.SplitN(ansi.Strip(m.View()), "\n", 2)[0]; ansi.StringWidth(first) > 30 || !strings.Contains(first, "CS") {
		t.Fatalf("narrow title broken: %q", first)
	}
	m.Width, m.Height = 120, 30
	press(m, tea.KeyF1)
	if !strings.Contains(ansi.Strip(m.View()), Version) {
		t.Fatal("F1 help lacks the version")
	}
	press(m, tea.KeyF1)
	press(m, tea.KeyF2)
	if first := strings.SplitN(ansi.Strip(m.View()), "\n", 2)[0]; !strings.Contains(first, "Edit command") || !strings.Contains(first, Version) {
		t.Fatalf("editor header lacks the version: %q", first)
	}
}

func TestDiscardDialogOverEditor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lib := fixtureLibrary(t, "settings: {color: always, theme: catppuccin-mocha}\nsnippets:\n  - name: Analyze logs\n    command: echo hi\n")
	open := func(t *testing.T) *Model {
		t.Helper()
		m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0]})
		if err != nil {
			t.Fatal(err)
		}
		m.Width, m.Height = 120, 30
		m.Editor.Description.Set("changed")
		m.Editor.Build()
		press(m, tea.KeyEsc)
		if !m.ConfirmDiscard {
			t.Fatal("Esc on a dirty editor did not ask")
		}
		return m
	}
	m := open(t)
	view := m.View()
	plain := ansi.Strip(view)
	for _, want := range []string{"Discard unsaved changes?", "“Analyze logs”", "Discard", "Keep editing", "←→ choose", "Enter confirm", "Friendly name"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("dialog missing %q:\n%s", want, plain)
		}
	}
	// A framed box, drawn over (not instead of) the editor.
	lines := strings.Split(plain, "\n")
	boxRow := -1
	for i, line := range lines {
		if strings.Contains(line, "Discard unsaved changes?") {
			boxRow = i
		}
	}
	if boxRow < 3 || boxRow > len(lines)-4 || !strings.Contains(lines[boxRow], "╭") {
		t.Fatalf("dialog is not a centered framed box (row %d):\n%s", boxRow, plain)
	}
	// Keep editing is the default; the two choices look different.
	if m.DiscardChoice != 1 {
		t.Fatalf("default choice should be Keep editing, got %d", m.DiscardChoice)
	}
	if !strings.Contains(view, m.Styles.ActiveRow.Bold(true).Render(" Keep editing ")) {
		t.Fatalf("selected Keep editing is not highlighted:\n%q", view)
	}
	press(m, tea.KeyEnter) // Enter on the default keeps editing
	if m.ConfirmDiscard || m.Mode != "editor" {
		t.Fatal("Enter on Keep editing did not return to the editor")
	}
	m = open(t)
	press(m, tea.KeyLeft) // choose Discard
	selectedDanger := m.Styles.Renderer.NewStyle().Reverse(true).Inherit(m.Styles.Error).Render(" Discard ")
	if m.DiscardChoice != 0 || !strings.Contains(m.View(), selectedDanger) {
		t.Fatalf("Discard is not selected/tinted: %d", m.DiscardChoice)
	}
	press(m, tea.KeyEnter)
	if m.ConfirmDiscard || m.Mode == "editor" {
		t.Fatal("Enter on Discard did not discard")
	}
	// y / n / Esc shortcuts still work.
	m = open(t)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.ConfirmDiscard || m.Mode != "editor" {
		t.Fatal("n did not keep editing")
	}
	m = open(t)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.Mode == "editor" {
		t.Fatal("y did not discard")
	}
	// Without colour, brackets show which button is selected.
	plainLib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: Analyze logs\n    command: echo hi\n")
	pm, _ := New(plainLib, Options{Start: "edit", Entry: plainLib.Entries[0]})
	pm.Width, pm.Height = 120, 30
	pm.Editor.Description.Set("changed")
	pm.Editor.Build()
	press(pm, tea.KeyEsc)
	if v := pm.View(); strings.Contains(v, "\x1b") || !strings.Contains(v, "[Keep editing]") || strings.Contains(v, "[Discard]") {
		t.Fatalf("no-color dialog does not mark the selection:\n%s", v)
	}
	press(pm, tea.KeyLeft)
	if v := pm.View(); !strings.Contains(v, "[Discard]") || strings.Contains(v, "[Keep editing]") {
		t.Fatalf("no-color selection did not move:\n%s", v)
	}
	// Small terminals and no-color still render a readable prompt.
	m = open(t)
	for _, size := range [][2]int{{60, 18}, {40, 10}, {24, 6}} {
		m.Width, m.Height = size[0], size[1]
		v := ansi.Strip(m.View())
		if !strings.Contains(v, "Discard") || !strings.Contains(v, "Keep") {
			t.Fatalf("%v dialog unreadable:\n%s", size, v)
		}
		for _, line := range strings.Split(v, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("%v dialog overflows: %q", size, line)
			}
		}
	}
}
