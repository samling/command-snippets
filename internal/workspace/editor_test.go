package workspace

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
		if strings.Contains(c.Label, "Validation pattern") || strings.Contains(c.Label, "Use integer range") || strings.Contains(c.Label, "must be a regex") {
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
