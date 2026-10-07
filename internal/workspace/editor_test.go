package workspace

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/templating"
	"gopkg.in/yaml.v3"
)

func TestQuotedWholeSpanAndToggleEditorControls(t *testing.T) {
	e, err := NewEditor(nil, "main.yaml")
	if err != nil {
		t.Fatal(err)
	}
	e.Name.Set("Quoted")
	e.Command.Set("echo 'hello world'")
	e.Command.Cursor = 5
	e.Mark()
	e.Command.Cursor = len(e.Command.Value)
	e.Mark()
	e.PickerQuotes = true
	e.confirmMark()
	draft, err := e.Draft()
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := templating.Compile(draft)
	if err != nil {
		t.Fatal(err)
	}
	command, err := compiled.Render(nil)
	if err != nil || command != "echo 'hello world'" {
		t.Fatalf("whole quoted selection %q %v", command, err)
	}
	e, err = NewEditor(&library.Entry{Source: &library.Source{Path: "main.yaml"}, Snippet: models.Snippet{Name: "Toggle", Command: "echo {{enabled}}", Inputs: []models.Input{{Name: "enabled", Kind: "toggle", Flag: "-x", Required: true}}}}, "main.yaml")
	if err != nil {
		t.Fatal(err)
	}
	e.Section = 1
	e.Build()
	for _, c := range e.Controls {
		if strings.Contains(c.Label, "Must match") || strings.Contains(c.Label, "Number range") || strings.Contains(c.Label, "Must be regex") {
			t.Fatal("toggle content validation exposed")
		}
	}
	for _, size := range [][2]int{{90, 24}, {40, 10}, {24, 6}} {
		if view := e.View(size[0], size[1]); !strings.Contains(view, "Ctrl-S:save") || !strings.Contains(view, "Esc:back") {
			t.Fatalf("editor footer clipped at %v: %q", size, view)
		}
	}
}
func TestEditorConditionRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		condition  *models.Condition
	}{
		{"none", "text", nil},
		{"expression", "text", &models.Condition{Expr: `inputs.controller == "show"`}},
		{"text equals", "text", &models.Condition{Input: "controller", Equals: "show"}},
		{"text not equals", "text", &models.Condition{Input: "controller", NotEquals: ""}},
		{"toggle equals", "toggle", &models.Condition{Input: "controller", Equals: false}},
		{"repeat equals", "repeat", &models.Condition{Input: "controller", Equals: []string{"A=1", "B=hello world"}}},
		{"YAML repeat not equals", "repeat", &models.Condition{Input: "controller", NotEquals: []any{"A=1", "B=hello world"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := models.Input{Name: "controller"}
			if tc.kind != "text" {
				controller.Kind, controller.Flag = tc.kind, "-x"
			}
			snippet := models.Snippet{Name: "Conditions", Command: "echo {{value}}", Inputs: []models.Input{
				controller,
				{Name: "value", VisibleWhen: tc.condition, RequiredWhen: tc.condition},
			}}
			e, err := NewEditor(&library.Entry{Snippet: snippet, Source: &library.Source{DisplayPath: "main.yaml"}}, "")
			if err != nil {
				t.Fatal(err)
			}
			draft, err := e.Draft()
			if err != nil {
				t.Fatal(err)
			}
			before, err := yaml.Marshal(snippet)
			if err != nil {
				t.Fatal(err)
			}
			after, err := yaml.Marshal(draft)
			if err != nil || string(after) != string(before) {
				t.Fatalf("condition changed:\n%s\n%s\n%v", before, after, err)
			}
		})
	}
}

func TestColorSettingAndExplicitNoColor(t *testing.T) {
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: Test\n    command: echo\n")
	m, err := New(lib, Options{})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 90, 24
	if strings.Contains(m.View(), "\x1b") {
		t.Fatal("color never emitted escapes")
	}
	lib.Settings.Color = "always"
	m, err = New(lib, Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 90, 24
	if strings.Contains(m.View(), "\x1b") {
		t.Fatal("explicit no-color ignored")
	}
}
func TestEditorCtrlSSavesRatherThanEmits(t *testing.T) {
	lib := fixtureLibrary(t, "snippets: []\n")
	m, err := New(lib, Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	press(m, tea.KeyCtrlN)
	m.Editor.Name.Set("Saved")
	m.Editor.Command.Set("echo saved")
	press(m, tea.KeyCtrlS)
	if m.Command != "" || m.Mode != "library" || len(m.Library.Entries) != 1 {
		t.Fatal("editor Ctrl-S did not save without emission")
	}
}

func navigationEditor(t *testing.T, color bool) *Model {
	t.Helper()
	settings := models.Settings{Color: "never"}
	if color {
		settings = models.Settings{Color: "always", Theme: "catppuccin-mocha"}
	}
	m, err := New(&library.Library{Settings: settings}, Options{Start: "add"})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 120, 30
	m.Editor.Name.Set("Folder")
	m.Editor.Command.Set("ls {{folder_name}}")
	m.Editor.Build()
	return m
}

func focusEditorLabel(t *testing.T, e *Editor, label string) {
	t.Helper()
	for i, c := range e.Controls {
		if c.Label == label {
			e.Focus = i
			return
		}
	}
	t.Fatalf("control %q missing: %+v", label, e.Controls)
}

func TestEditorSectionActionsFocusFirstRightPaneControl(t *testing.T) {
	for _, tc := range []struct {
		action  string
		section int
	}{{"Configure inputs", 0}, {"Computed values", 2}, {"Test preview", 3}} {
		t.Run(tc.action, func(t *testing.T) {
			m := navigationEditor(t, false)
			e := m.Editor
			e.Expressions = []pairDraft{{text("extra"), text(`"x"`)}}
			e.Section = 2
			e.Build()
			focusEditorLabel(t, e, tc.action)
			press(m, tea.KeyEnter)
			if e.Section != tc.section || e.Focus != e.firstPaneControl() || e.Focus >= len(e.Controls) {
				t.Fatalf("Enter on %s left focus on %q (section %d)", tc.action, e.Controls[e.Focus].Label, e.Section)
			}
			switch tc.section {
			case 0:
				if e.Controls[e.Focus].Label != "folder_name" {
					t.Fatalf("inputs pane did not focus its first list row: %q", e.Controls[e.Focus].Label)
				}
			case 2:
				if e.Controls[e.Focus].Text != &e.Expressions[0].Key {
					t.Fatalf("computed values pane did not focus its first value: %q", e.Controls[e.Focus].Label)
				}
			case 3:
				if e.Test == nil || e.Controls[e.Focus].Text != e.Test.Texts["folder_name"] {
					t.Fatalf("test pane did not focus first test value: %q", e.Controls[e.Focus].Label)
				}
			}
		})
	}
}

func TestEditorArrowsMoveBetweenControlsAndMultilineCommandLines(t *testing.T) {
	m := navigationEditor(t, false)
	e := m.Editor
	e.Expressions = []pairDraft{{text("a"), text("x")}, {text("b"), text("y")}}
	e.Section = 2
	e.Build()
	first := e.firstPaneControl()
	e.Focus = first
	press(m, tea.KeyDown)
	if e.Focus != first+1 {
		t.Fatalf("Down stayed on %q", e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyUp)
	if e.Focus != first {
		t.Fatalf("Up did not return: %q", e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyLeft)
	if e.Focus != first || e.Expressions[0].Key.Cursor != 0 {
		t.Fatalf("Left left the text control or ignored its cursor: focus=%q cursor=%d", e.Controls[e.Focus].Label, e.Expressions[0].Key.Cursor)
	}
	e.Command.Set("echo one\necho {{folder_name}}")
	e.Command.Cursor = len(e.Command.Value)
	e.Build()
	focusEditorLabel(t, e, "Command")
	press(m, tea.KeyUp)
	if e.Controls[e.Focus].Text != &e.Command || e.Command.Cursor > len("echo one") {
		t.Fatalf("Up on a later Command line moved focus instead of the cursor: %q cursor=%d", e.Controls[e.Focus].Label, e.Command.Cursor)
	}
	press(m, tea.KeyUp)
	if e.Controls[e.Focus].Text != &e.Name {
		t.Fatalf("Up on the first Command line did not move focus: %q", e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyDown)
	press(m, tea.KeyDown)
	if e.Controls[e.Focus].Text != &e.Command || !strings.Contains(string(e.Command.Value[:e.Command.Cursor]), "\n") {
		t.Fatalf("Down on the first Command line did not move the cursor: %q", e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyDown)
	if e.Controls[e.Focus].Text != &e.Description {
		t.Fatalf("Down on the last Command line did not move focus: %q", e.Controls[e.Focus].Label)
	}
}

func TestEditorInputListOpensDetailsAndReturns(t *testing.T) {
	m := navigationEditor(t, false)
	e := m.Editor
	e.Command.Set("ls {{folder_name}} {{port}}")
	e.Build()
	e.Inputs[1].Kind, e.Inputs[1].Required = "flag", false
	e.Inputs[1].Flag.Set("-p")
	e.Build()
	plain := m.View()
	for _, want := range []string{"folder_name", "text · required", "port", "flag -p", "+ Add input"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("input list row %q missing: %s", want, plain)
		}
	}
	if strings.Contains(plain, "Selected input") || strings.Contains(plain, "definitions stay") || strings.Contains(plain, "Validation pattern") {
		t.Fatalf("list shows selector or details: %s", plain)
	}
	focusEditorLabel(t, e, "Configure inputs")
	press(m, tea.KeyEnter)
	press(m, tea.KeyDown)
	if e.Controls[e.Focus].Label != "port" {
		t.Fatalf("Down did not reach the second input row: %q", e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyEnter)
	port := e.Inputs[1]
	view := m.View()
	if e.Controls[e.Focus].Text != &port.Name || !strings.Contains(view, "Input · port") || !strings.Contains(view, "‹ Back to inputs") || !strings.Contains(view, "Must match") {
		t.Fatalf("Enter did not open port details with Name focused: %q\n%s", e.Controls[e.Focus].Label, view)
	}
	press(m, tea.KeyEsc)
	if m.Mode != "editor" || m.ConfirmDiscard || e.Controls[e.Focus].Label != "port" || strings.Contains(m.View(), "Validation pattern") {
		t.Fatalf("Esc in details did not return to the port row: mode=%s confirm=%v focus=%q", m.Mode, m.ConfirmDiscard, e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyEnter)
	press(m, tea.KeyUp)
	if e.Controls[e.Focus].Label != "‹ Back to inputs" {
		t.Fatalf("Back row is not first in details: %q", e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyEnter)
	if e.Controls[e.Focus].Label != "port" {
		t.Fatalf("Back did not focus the port row: %q", e.Controls[e.Focus].Label)
	}
	focusEditorLabel(t, e, "+ Add input")
	press(m, tea.KeyEnter)
	if len(e.Inputs) != 3 || e.Controls[e.Focus].Text != &e.Inputs[2].Name || !strings.Contains(m.View(), "Input · input") {
		t.Fatalf("Add input did not open the new input details: %d %q", len(e.Inputs), e.Controls[e.Focus].Label)
	}
	focusEditorLabel(t, e, "Remove this input")
	press(m, tea.KeyEnter)
	if len(e.Inputs) != 2 || e.Focus < e.BasicsCount || strings.Contains(m.View(), "Validation pattern") || !strings.Contains(m.View(), "+ Add input") {
		t.Fatalf("Remove did not return to the list: %d %q", len(e.Inputs), e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyEsc) // first Esc leaves the right pane for its opener
	if m.ConfirmDiscard || e.Controls[e.Focus].Label != "Configure inputs" {
		t.Fatalf("Esc in the input list should return to Configure inputs: %q", e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyEsc)
	if !m.ConfirmDiscard {
		t.Fatal("Esc from the left pane lost dirty back confirmation")
	}
}

func TestEditorAlignedRowsShowOnlyFocusedHint(t *testing.T) {
	m := navigationEditor(t, false)
	e := m.Editor
	focusEditorLabel(t, e, "Configure inputs")
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	focusEditorLabel(t, e, "Must match")
	view := m.View()
	if strings.Contains(view, "Name:") || strings.Contains(view, "Required:") || strings.Contains(view, "Friendly name:") {
		t.Fatalf("Label: value rows remain: %s", view)
	}
	if strings.Count(view, "A regular expression non-empty values must match") != 1 || strings.Contains(view, "A searchable display title") {
		t.Fatalf("help is not limited to the focused field hint: %s", view)
	}
	columns := map[string]int{}
	for _, line := range strings.Split(view, "\n") {
		_, right, ok := strings.Cut(line, "││")
		if !ok {
			continue
		}
		for _, label := range []string{"Name", "Description", "Required"} {
			if index := strings.Index(right, "  "+label+"  "); index >= 0 {
				rest := right[index+2+len(label):]
				columns[label] = index + 2 + len(label) + len(rest) - len(strings.TrimLeft(rest, " "))
			}
		}
	}
	if len(columns) != 3 || columns["Name"] != columns["Description"] || columns["Name"] != columns["Required"] {
		t.Fatalf("detail values are not in one column %v: %s", columns, view)
	}
	hint := -1
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "A regular expression non-empty values must") && !strings.Contains(line, "Must match ·") {
			hint = i
		}
	}
	if hint < 0 {
		t.Fatalf("focused hint missing: %s", view)
	}
}

func TestEditorTestPaneExplainsUnfocusedFieldErrors(t *testing.T) {
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets: []\n")
	m, err := New(lib, Options{Start: "add"})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 120, 30
	m.Editor.Name.Set("Pattern")
	m.Editor.Command.Set("echo {{value}} {{other}}")
	m.Editor.Build()
	for _, in := range m.Editor.Inputs {
		if in.Name.String() == "value" {
			in.Pattern.Set("^[0-9]+$")
		}
	}
	for i, c := range m.Editor.Controls {
		if c.Label == "Test preview" {
			m.Editor.Focus = i
		}
	}
	press(m, tea.KeyEnter)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})
	press(m, tea.KeyTab)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "value: must match ^[0-9]+$") {
		t.Fatalf("unfocused Test validation reason absent:\n%s", view)
	}
	if strings.Contains(view, "is required") || !strings.Contains(view, "‹other›") {
		t.Fatalf("empty required Test field should be marked quietly, not as an error:\n%s", view)
	}
}

func TestEditorShiftTabFromPaneReturnsToItsOpener(t *testing.T) {
	for _, tc := range []struct{ action string }{{"Configure inputs"}, {"Computed values"}, {"Test preview"}} {
		t.Run(tc.action, func(t *testing.T) {
			m := navigationEditor(t, false)
			e := m.Editor
			e.Expressions = []pairDraft{{text("extra"), text(`"x"`)}}
			e.Build()
			focusEditorLabel(t, e, tc.action)
			press(m, tea.KeyEnter)
			if e.Focus < e.BasicsCount {
				t.Fatalf("Enter did not open the pane")
			}
			press(m, tea.KeyShiftTab)
			if e.Controls[e.Focus].Label != tc.action {
				t.Fatalf("Shift-Tab from the pane's first row went to %q, want %q", e.Controls[e.Focus].Label, tc.action)
			}
			press(m, tea.KeyEnter)
			press(m, tea.KeyUp)
			if e.Controls[e.Focus].Label != tc.action {
				t.Fatalf("Up from the pane's first row went to %q, want %q", e.Controls[e.Focus].Label, tc.action)
			}
		})
	}
}

func TestComputedValuesExplainedAndAddFocusesFormula(t *testing.T) {
	m := navigationEditor(t, false)
	e := m.Editor
	e.Command.Set("awk {{awk_script}} {{folder_name}}")
	e.Expressions = []pairDraft{{text("awk_script"), text(`quote("/" + inputs.folder_name + "/")`)}}
	e.Build()
	focusEditorLabel(t, e, "Computed values")
	press(m, tea.KeyEnter)
	view := ansi.Strip(m.View())
	for _, want := range []string{"Computed values", "Formula", "uses folder_name", "+ Add computed value"} {
		if !strings.Contains(view, want) {
			t.Fatalf("computed values pane missing %q:\n%s", want, view)
		}
	}
	// The explanation lives behind ?, not inline where it takes editing space.
	if strings.Contains(view, "builds one piece of the command") {
		t.Fatalf("explanation is inline instead of behind ?:\n%s", view)
	}
	if strings.Contains(view, "Expression string result") || strings.Contains(view, "Add Expression") {
		t.Fatalf("old jargon remains:\n%s", view)
	}
	focusEditorLabel(t, e, "+ Add computed value")
	press(m, tea.KeyEnter)
	if len(e.Expressions) != 2 || e.Controls[e.Focus].Text != &e.Expressions[1].Value {
		t.Fatalf("Add did not focus the new value's formula; focus %q", e.Controls[e.Focus].Label)
	}
	// A fresh formula is empty (no ambiguous starter quotes) with a dim example.
	if e.Expressions[1].Key.String() == "" || e.Expressions[1].Value.String() != "" {
		t.Fatalf("new computed value: name %q formula %q", e.Expressions[1].Key.String(), e.Expressions[1].Value.String())
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "e.g. quote(inputs.folder_name)") || !strings.Contains(view, "write a formula") || strings.Contains(view, `Formula                ""`) {
		t.Fatalf("empty formula lacks its example/reminder:\n%s", view)
	}
	if e.Error != "" || e.CompileError != "" {
		t.Fatalf("an empty new formula should not raise an error banner: %q %q", e.Error, e.CompileError)
	}
	// Notes are read-only: arrows step over them.
	before := e.Focus
	press(m, tea.KeyDown)
	if e.Controls[e.Focus].Note {
		t.Fatalf("focus landed on a note row after %d", before)
	}
}

func TestTagsMenuFiltersTogglesAndCreates(t *testing.T) {
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: A\n    command: echo a\n    tags: [kubernetes, networking]\n  - name: B\n    command: echo b\n    tags: [kubernetes]\n  - name: C\n    command: echo c\n    tags: [docker]\n")
	if lib.Error() != nil {
		t.Fatal(lib.Error())
	}
	m, err := New(lib, Options{Start: "add"})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 120, 30
	e := m.Editor
	focusEditorLabel(t, e, "Tags")
	press(m, tea.KeyEnter)
	view := ansi.Strip(m.View())
	for _, want := range []string{"[ ] kubernetes", "(2)", "[ ] docker", "[ ] networking"} {
		if !strings.Contains(view, want) {
			t.Fatalf("tag menu missing %q:\n%s", want, view)
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kub")})
	press(m, tea.KeyEnter)
	if got := strings.Fields(e.TagsText.String()); len(got) != 1 || got[0] != "kubernetes" {
		t.Fatalf("Enter did not choose filtered kubernetes: %q", e.TagsText.String())
	}
	if !strings.Contains(ansi.Strip(m.View()), "[x] kubernetes") {
		t.Fatalf("chosen tag not checked:\n%s", ansi.Strip(m.View()))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("logs")})
	if !strings.Contains(ansi.Strip(m.View()), "+ create “logs”") {
		t.Fatalf("create option missing:\n%s", ansi.Strip(m.View()))
	}
	press(m, tea.KeyEnter)
	if got := strings.Fields(e.TagsText.String()); len(got) != 2 || got[1] != "logs" {
		t.Fatalf("create did not add logs: %q", e.TagsText.String())
	}
	// Backspace never removes chosen tags: with nothing typed it does nothing.
	press(m, tea.KeyBackspace)
	if got := strings.Fields(e.TagsText.String()); len(got) != 2 {
		t.Fatalf("Backspace with nothing typed changed the tags: %q", e.TagsText.String())
	}
	// It only deletes characters typed into the filter.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("doc")})
	press(m, tea.KeyBackspace)
	if got := strings.Fields(e.TagsText.String()); len(got) != 3 || got[2] != "do" {
		t.Fatalf("Backspace did not edit the typed filter: %q", e.TagsText.String())
	}
	for i := 0; i < 4; i++ { // clear the filter, then keep pressing
		press(m, tea.KeyBackspace)
	}
	if got := strings.Fields(e.TagsText.String()); len(got) != 2 || got[0] != "kubernetes" || got[1] != "logs" {
		t.Fatalf("extra Backspaces removed chosen tags: %q", e.TagsText.String())
	}
	// Untick with Enter on the tag's row.
	for i := 0; i < 10 && e.tagItems()[e.TagMenu].Label != "logs"; i++ {
		press(m, tea.KeyDown)
	}
	press(m, tea.KeyEnter)
	if got := strings.Fields(e.TagsText.String()); len(got) != 1 {
		t.Fatalf("Enter did not untick logs: %q", e.TagsText.String())
	}
	// A just-created tag stays listed (marked new) until it is unticked.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("logs")})
	press(m, tea.KeyEnter)
	for i := 0; i < 4; i++ {
		press(m, tea.KeyBackspace)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "[x] logs") || !strings.Contains(view, "new") {
		t.Fatalf("created tag missing from the list after clearing the filter:\n%s", view)
	}
	for i := 0; i < 10 && e.tagItems()[e.TagMenu].Label != "logs"; i++ {
		press(m, tea.KeyDown)
	}
	press(m, tea.KeyEnter)
	if view := ansi.Strip(m.View()); strings.Contains(view, "Backspace") || !strings.Contains(view, "Enter tick/untick") {
		t.Fatalf("checklist hint is wrong:\n%s", view)
	}
	press(m, tea.KeyTab)
	if e.Controls[e.Focus].ID == tagsID {
		t.Fatal("Tab did not leave Tags")
	}
	if strings.Contains(ansi.Strip(m.View()), "[ ] docker") {
		t.Fatal("menu still open after leaving Tags")
	}
	draft, err := e.Draft()
	if err != nil || len(draft.Tags) != 1 || draft.Tags[0] != "kubernetes" {
		t.Fatalf("saved tags wrong: %v %v", draft.Tags, err)
	}
}

func TestTagsFocusStartsAFreshWordAndListsAllTags(t *testing.T) {
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: A\n    command: echo a\n    tags: [logs, text]\n  - name: B\n    command: echo b\n    tags: [kubernetes]\n")
	m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0]})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 120, 30
	e := m.Editor
	focusEditorLabel(t, e, "Description")
	press(m, tea.KeyTab) // arrive on Tags the way a user does (arrows drive its menu)
	if e.Controls[e.Focus].ID != tagsID {
		t.Fatalf("not on Tags: %q", e.Controls[e.Focus].Label)
	}
	view := ansi.Strip(m.View())
	for _, want := range []string{"[x] logs", "[x] text", "[ ] kubernetes"} {
		if !strings.Contains(view, want) {
			t.Fatalf("focused Tags menu missing %q (must list all tags, not filter by the last chosen one):\n%s", want, view)
		}
	}
	press(m, tea.KeyEnter)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kub")})
	if got := strings.Fields(e.TagsText.String()); len(got) != 3 || got[2] != "kub" {
		t.Fatalf("typing joined the previous tag: %q", e.TagsText.String())
	}
	// Merely visiting Tags must not edit them (legacy multiword tags stay intact).
	m2, _ := New(lib, Options{Start: "edit", Entry: lib.Entries[0]})
	focusEditorLabel(t, m2.Editor, "Description")
	press(m2, tea.KeyTab)
	press(m2, tea.KeyTab)
	if m2.Editor.TagsText.String() != "logs text" || m2.Editor.Dirty() {
		t.Fatalf("visiting Tags changed them: %q dirty=%v", m2.Editor.TagsText.String(), m2.Editor.Dirty())
	}
}

func TestTagsMenuOpensInRightPaneKeepingTheForm(t *testing.T) {
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: A\n    command: echo a\n    tags: [logs, text]\n  - name: B\n    command: echo b\n    tags: [kubernetes]\n")
	for _, size := range [][2]int{{120, 30}, {80, 30}} {
		m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0]})
		if err != nil {
			t.Fatal(err)
		}
		m.Width, m.Height = size[0], size[1]
		focusEditorLabel(t, m.Editor, "Description")
		press(m, tea.KeyTab)
		view := ansi.Strip(m.View())
		for _, want := range []string{"Configure inputs", "Computed values", "Test preview", "[ ] kubernetes", "Tab: next field"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%v: with Tags focused, %q missing:\n%s", size, want, view)
			}
		}
		// The checklist sits in the right/second pane, not on top of the left one.
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, "Configure inputs") && strings.Contains(line, "[ ]") && size[0] < 90 {
				t.Fatalf("checklist overlaps the form row: %q", line)
			}
		}
		if size[0] >= 90 {
			for _, line := range strings.Split(view, "\n") {
				if i := strings.Index(line, "kubernetes"); i >= 0 && i < size[0]/2 {
					t.Fatalf("checklist is in the left pane: %q", line)
				}
			}
		}
	}
}

func TestComputedValuesExplainFormulasAndSeparateEntries(t *testing.T) {
	m := navigationEditor(t, false)
	e := m.Editor
	e.Command.Set("awk {{awk_script}} {{folder_name}}")
	e.Expressions = []pairDraft{{text("awk_script"), text(`quote("/" + inputs.folder_name + "/")`)}, {text("dir_arg"), text("inputs.folder_name")}}
	e.Build()
	focusEditorLabel(t, e, "Computed values")
	press(m, tea.KeyEnter)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	// The help is scrollable; collect every page the user can reach with Down.
	help := ansi.Strip(m.View())
	for i := 0; i < 40; i++ {
		press(m, tea.KeyDown)
		help += ansi.Strip(m.View())
	}
	// Help reflows to the pane width, so compare with borders and line breaks collapsed.
	flat := strings.Join(strings.Fields(strings.ReplaceAll(help, "│", " ")), " ")
	for _, want := range []string{"Help · Computed values", "the Command field", "inputs.pattern", `"text"`, "quote(", "flag(", "default(", "a ? b : c", "exactly as written", "Examples:"} {
		if !strings.Contains(flat, want) {
			t.Fatalf("? help missing %q:\n%s", want, help)
		}
	}
	if !e.PaneHelp || e.Expressions[0].Key.String() != "awk_script" {
		t.Fatal("scrolling help closed it or changed the field")
	}
	if e.Expressions[0].Key.String() != "awk_script" {
		t.Fatalf("? was typed into the Name field: %q", e.Expressions[0].Key.String())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	view := ansi.Strip(m.View())
	if strings.Contains(view, "Help · ") {
		t.Fatalf("? did not close help:\n%s", view)
	}
	// Each computed value starts with a visible separator naming it.
	if strings.Count(view, "── awk_script") != 1 || strings.Count(view, "── dir_arg") != 1 {
		t.Fatalf("computed values are not visually separated:\n%s", view)
	}
	// Separators are not focusable.
	for range e.Controls {
		press(m, tea.KeyDown)
		if c := e.Controls[e.Focus]; c.Note {
			t.Fatalf("focus landed on read-only row %q", c.Label)
		}
	}
}

func TestTagsChecklistNeedsEnterSoArrowsScrollPast(t *testing.T) {
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: A\n    command: echo a\n    tags: [logs, text]\n  - name: B\n    command: echo b\n    tags: [kubernetes]\n")
	m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0]})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 120, 30
	e := m.Editor
	focusEditorLabel(t, e, "Description")
	press(m, tea.KeyDown)
	if e.Controls[e.Focus].ID != tagsID || e.TagEditing {
		t.Fatalf("Down should land on Tags without entering the checklist: %q editing=%v", e.Controls[e.Focus].Label, e.TagEditing)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "[ ] kubernetes") || !strings.Contains(view, "Enter: edit tags") {
		t.Fatalf("checklist preview or Enter hint missing:\n%s", view)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zz")})
	if e.TagsText.String() != "logs text" {
		t.Fatalf("typing changed tags without Enter: %q", e.TagsText.String())
	}
	press(m, tea.KeyDown)
	if e.Controls[e.Focus].Label != "Configure inputs" {
		t.Fatalf("Down did not continue past Tags: %q", e.Controls[e.Focus].Label)
	}
	press(m, tea.KeyUp)
	press(m, tea.KeyEnter)
	if !e.TagEditing || e.Controls[e.Focus].ID != tagsID {
		t.Fatal("Enter on Tags did not open the checklist")
	}
	press(m, tea.KeyDown)
	if e.Controls[e.Focus].ID != tagsID || e.TagMenu != 1 {
		t.Fatalf("Down inside the checklist moved focus instead of the pick: %q menu=%d", e.Controls[e.Focus].Label, e.TagMenu)
	}
	press(m, tea.KeyEsc)
	if e.TagEditing || m.ConfirmDiscard || m.Mode != "editor" || e.Controls[e.Focus].ID != tagsID {
		t.Fatalf("Esc should leave the checklist only: editing=%v confirm=%v mode=%s", e.TagEditing, m.ConfirmDiscard, m.Mode)
	}
	press(m, tea.KeyEnter)
	press(m, tea.KeyTab)
	if e.TagEditing || e.Controls[e.Focus].Label != "Configure inputs" {
		t.Fatalf("Tab should leave the checklist to the next field: %q editing=%v", e.Controls[e.Focus].Label, e.TagEditing)
	}
}

func TestHelpKeyTypesIntoFormulasAndEscClosesHelp(t *testing.T) {
	m := navigationEditor(t, false)
	e := m.Editor
	e.Expressions = []pairDraft{{text("pick"), text("inputs.folder_name")}}
	e.Build()
	focusEditorLabel(t, e, "Computed values")
	press(m, tea.KeyEnter)
	press(m, tea.KeyDown) // Formula: free text, so ? is a literal character
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !strings.HasSuffix(e.Expressions[0].Value.String(), "?") || e.PaneHelp {
		t.Fatalf("? in a formula must type, not open help: %q help=%v", e.Expressions[0].Value.String(), e.PaneHelp)
	}
	press(m, tea.KeyF1) // F1 opens help from inside a free-text field
	if !e.PaneHelp || !strings.Contains(ansi.Strip(m.View()), "Help · Computed values") {
		t.Fatal("F1 in a formula did not open the computed values help")
	}
	press(m, tea.KeyF1)
	if e.PaneHelp {
		t.Fatal("F1 did not close help")
	}
	focusEditorLabel(t, e, "Configure inputs")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !e.PaneHelp {
		t.Fatal("? on an action row did not open help")
	}
	press(m, tea.KeyEsc)
	if e.PaneHelp || m.ConfirmDiscard || m.Mode != "editor" {
		t.Fatalf("Esc should only close help: help=%v confirm=%v", e.PaneHelp, m.ConfirmDiscard)
	}
}

func TestDuplicateInputAndComputedNamesAreRejected(t *testing.T) {
	m := navigationEditor(t, false)
	e := m.Editor
	e.Command.Set("ls {{folder_name}} {{pick}}")
	e.Expressions = []pairDraft{{text("pick"), text("inputs.folder_name")}, {text("pick"), text(`"x"`)}}
	e.Build()
	if _, err := e.Draft(); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("duplicate computed names accepted: %v", err)
	}
	focusEditorLabel(t, e, "Computed values")
	press(m, tea.KeyEnter)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "name already used by another computed value") {
		t.Fatalf("duplicate computed name not flagged inline:\n%s", view)
	}
	press(m, tea.KeyCtrlS)
	if m.Mode != "editor" || !strings.Contains(e.Error, "already used") {
		t.Fatalf("save went ahead with duplicate names: mode=%s err=%q", m.Mode, e.Error)
	}
	// A computed value may not reuse an input's name either.
	e.Expressions = []pairDraft{{text("folder_name"), text(`"x"`)}}
	e.Build()
	if _, err := e.Draft(); err == nil || !strings.Contains(err.Error(), "already used by an input") {
		t.Fatalf("computed value shadowing an input accepted: %v", err)
	}
	// Two inputs with the same name (renamed in details) are rejected and marked on the list.
	e.Expressions = nil
	e.addInput()
	e.Build()
	e.Inputs[1].Name.Set("folder_name")
	e.closeInput()
	e.Section = 0
	e.Build()
	if _, err := e.Draft(); err == nil || !strings.Contains(err.Error(), "already used by another input") {
		t.Fatalf("duplicate input names accepted: %v", err)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "already used") {
		t.Fatalf("duplicate input not flagged in the list:\n%s", view)
	}
}

func TestEscFromRightPaneReturnsToItsOpenerBeforeQuitting(t *testing.T) {
	for _, tc := range []struct{ action string }{{"Configure inputs"}, {"Computed values"}, {"Test preview"}} {
		t.Run(tc.action, func(t *testing.T) {
			m := navigationEditor(t, false)
			e := m.Editor
			e.Expressions = []pairDraft{{text("extra"), text(`"x"`)}}
			e.Build()
			focusEditorLabel(t, e, tc.action)
			press(m, tea.KeyEnter)
			press(m, tea.KeyDown)
			if e.Focus < e.BasicsCount {
				t.Fatal("not in the right pane")
			}
			press(m, tea.KeyEsc)
			if m.ConfirmDiscard || m.Mode != "editor" || e.Controls[e.Focus].Label != tc.action {
				t.Fatalf("Esc in the right pane: focus %q confirm=%v", e.Controls[e.Focus].Label, m.ConfirmDiscard)
			}
			press(m, tea.KeyEsc)
			if !m.ConfirmDiscard {
				t.Fatal("Esc in the left pane should then ask to discard")
			}
		})
	}
	// Input details: Esc goes to the list first, then to the opener, then asks.
	m := navigationEditor(t, false)
	e := m.Editor
	focusEditorLabel(t, e, "Configure inputs")
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEsc)
	if e.OpenInput != nil || e.Focus < e.BasicsCount {
		t.Fatal("first Esc should close the input's details")
	}
	press(m, tea.KeyEsc)
	if e.Controls[e.Focus].Label != "Configure inputs" || m.ConfirmDiscard {
		t.Fatalf("second Esc should return to Configure inputs: %q", e.Controls[e.Focus].Label)
	}
}

func TestTagToggleKeepsHighlightAndShiftTabLeavesChecklist(t *testing.T) {
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: A\n    command: echo a\n    tags: [logs]\n  - name: B\n    command: echo b\n    tags: [kubernetes, docker]\n  - name: C\n    command: echo c\n    tags: [kubernetes, networking]\n")
	m, err := New(lib, Options{Start: "edit", Entry: lib.Entries[0]})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 120, 30
	e := m.Editor
	focusEditorLabel(t, e, "Tags")
	press(m, tea.KeyEnter)
	press(m, tea.KeyDown)
	press(m, tea.KeyDown)
	start := e.tagItems()[e.TagMenu]
	if e.TagMenu == 0 {
		t.Fatal("test needs a highlight below the first row")
	}
	for i := 1; i <= 2; i++ { // toggle twice: the highlight must stay put both times
		press(m, tea.KeyEnter)
		got := e.tagItems()[e.TagMenu]
		if got.Label != start.Label || got.Checked != (start.Checked != (i%2 == 1)) {
			t.Fatalf("toggle %d of %q: highlight on %q (row %d, checked=%v)", i, start.Label, got.Label, e.TagMenu, got.Checked)
		}
	}
	// Ticking a filtered match keeps it highlighted once the filter clears.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("netw")})
	press(m, tea.KeyEnter)
	if got := e.tagItems()[e.TagMenu]; got.Label != "networking" || !got.Checked {
		t.Fatalf("after ticking a filtered tag the highlight is on %q", got.Label)
	}
	// Shift-Tab leaves the checklist and stays on Tags in the left pane.
	press(m, tea.KeyShiftTab)
	if e.TagEditing || e.Controls[e.Focus].ID != tagsID {
		t.Fatalf("Shift-Tab from the checklist went to %q (editing=%v)", e.Controls[e.Focus].Label, e.TagEditing)
	}
	// From the Tags row itself, Shift-Tab moves up as usual.
	press(m, tea.KeyShiftTab)
	if e.Controls[e.Focus].Label != "Description" {
		t.Fatalf("Shift-Tab from the Tags row went to %q", e.Controls[e.Focus].Label)
	}
}

func openInputDetails(t *testing.T, m *Model, name string) *Editor {
	t.Helper()
	e := m.Editor
	for _, in := range e.Inputs {
		if in.Name.String() == name {
			e.openInput(in)
		}
	}
	e.Section = 0
	e.Build()
	if e.OpenInput == nil {
		t.Fatalf("input %s missing", name)
	}
	return e
}

func TestInputSettingsUsePlainLabelsSectionsAndAdvanced(t *testing.T) {
	m := navigationEditor(t, false)
	e := openInputDetails(t, m, "folder_name")
	view := ansi.Strip(m.View())
	for _, want := range []string{"── Basics", "Name", "Shown as", "Description", "Type", "Required", "── Value", "Starting value", "── Validation", "Must match", "Number range", "Must be regex", "Advanced ▸", "Remove this input"} {
		if !strings.Contains(view, want) {
			t.Fatalf("input settings missing %q:\n%s", want, view)
		}
	}
	for _, old := range []string{"Add Special", "Use default", "Validation pattern", "Value must be a regex", "Integer range", "Visible when", "Label  ", "Help  "} {
		if strings.Contains(view, old) {
			t.Fatalf("old label %q remains:\n%s", old, view)
		}
	}
	// Advanced is collapsed until opened, and then holds substitutions and conditions.
	if strings.Contains(view, "substitution") && !strings.Contains(view, "Advanced ▸") || strings.Contains(view, "Show only when") {
		t.Fatalf("advanced options are not collapsed:\n%s", view)
	}
	focusEditorLabel(t, e, "Advanced ▸")
	press(m, tea.KeyEnter)
	view = ansi.Strip(m.View())
	for _, want := range []string{"Advanced ▾", "+ Add substitution", "Show only when", "Required when", "always", "never"} {
		if !strings.Contains(view, want) {
			t.Fatalf("opened advanced missing %q:\n%s", want, view)
		}
	}
	// + Add substitution creates a row and focuses its first field.
	focusEditorLabel(t, e, "+ Add substitution")
	press(m, tea.KeyEnter)
	if c := e.Controls[e.Focus]; c.Label != "When typed" || len(e.OpenInput.Special) != 1 {
		t.Fatalf("add substitution focused %q", c.Label)
	}
	// Every editable row explains itself in the focus hint.
	for _, c := range e.Controls[e.BasicsCount:] {
		if c.Note || c.LabelOnly {
			continue
		}
		if strings.TrimSpace(c.Help) == "" {
			t.Errorf("%q has no hint", c.Label)
		}
	}
	// The list type is shown as "list" (stored as repeat).
	focusEditorLabel(t, e, "Type")
	for i := 0; i < 4; i++ {
		press(m, tea.KeyRight)
	}
	if e.OpenInput.Kind != "repeat" || !strings.Contains(ansi.Strip(m.View()), "<list>") {
		t.Fatalf("list maps to kind %q:\n%s", e.OpenInput.Kind, ansi.Strip(m.View()))
	}
}

func TestConfiguredAdvancedOpensAutomatically(t *testing.T) {
	m := navigationEditor(t, false)
	e := m.Editor
	e.Inputs[0].Special = []pairDraft{{text("all"), text("-A")}}
	e = openInputDetails(t, m, "folder_name")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Advanced ▾") || !strings.Contains(view, "When typed") {
		t.Fatalf("configured substitutions are hidden:\n%s", view)
	}
	draft, err := e.Draft()
	if err != nil || draft.Inputs[0].Special["all"] != "-A" {
		t.Fatalf("substitution not saved: %v %v", draft.Inputs[0].Special, err)
	}
}

func TestInputSettingsHelpIsReadable(t *testing.T) {
	m := navigationEditor(t, false)
	e := openInputDetails(t, m, "folder_name")
	_ = e
	press(m, tea.KeyF1)
	text := ""
	for i := 0; i < 30; i++ {
		text += ansi.Strip(m.View()) + "\n"
		press(m, tea.KeyDown)
	}
	right := []string{}
	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, "││"); i >= 0 {
			right = append(right, strings.Trim(line[i+len("││"):], "│ "))
		}
	}
	// Each setting name starts its own row; its description never runs into the next one.
	for _, term := range []string{"Name ", "Shown as ", "Description ", "Type ", "Required ", "Starting value ", "Must match ", "Number range ", "Must be regex ", "Substitutions ", "Show only when ", "Required when "} {
		found := false
		for _, line := range right {
			if strings.HasPrefix(line, term) {
				found = true
			}
			if idx := strings.Index(line, term); idx > 0 && strings.HasPrefix(term, "Shown") {
				t.Fatalf("%q runs into another row: %q", term, line)
			}
		}
		if !found {
			t.Fatalf("help has no row starting with %q:\n%s", term, strings.Join(right, "\n"))
		}
	}
}
