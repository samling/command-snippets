package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
func TestMarkChangesOnlyExactRangeAndUnknownOffersConfiguration(t *testing.T) {
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
	found := false
	for _, control := range e.Controls {
		if strings.Contains(control.Label, "Create undefined input") {
			found = true
			control.Action()
			break
		}
	}
	if !found || len(e.Inputs) != 2 || e.Section != 1 {
		t.Fatal("undefined input control unavailable")
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
