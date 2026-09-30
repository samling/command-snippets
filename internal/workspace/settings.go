package workspace

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
	forms "github.com/samling/command-snippets/internal/template"
	"gopkg.in/yaml.v3"
)

type SettingsEditor struct {
	Sources  []forms.Text
	Project  bool
	Default  forms.Text
	Color    string
	Focus    int
	Controls []forms.Control
	Initial  []byte
	Error    string
}

func NewSettingsEditor(lib *library.Library) *SettingsEditor {
	settings := lib.Settings
	if settings.Color == "" {
		settings = models.DefaultSettings()
	}
	e := &SettingsEditor{Project: settings.ProjectSource, Default: text(settings.DefaultSource), Color: settings.Color}
	for _, source := range settings.Sources {
		e.Sources = append(e.Sources, text(source))
	}
	initial, err := yaml.Marshal(settings)
	if err != nil {
		e.Error = err.Error()
	} else {
		e.Initial = initial
	}
	e.Build()
	return e
}
func (e *SettingsEditor) Draft() (models.Settings, error) {
	settings := models.Settings{ProjectSource: e.Project, DefaultSource: e.Default.String(), Color: e.Color}
	for _, source := range e.Sources {
		settings.Sources = append(settings.Sources, source.String())
	}
	return settings, settings.Validate()
}
func (e *SettingsEditor) Dirty() bool {
	settings, err := e.Draft()
	if err != nil {
		return true
	}
	data, err := yaml.Marshal(settings)
	return err != nil || string(data) != string(e.Initial)
}
func (e *SettingsEditor) Build() {
	e.Controls = nil
	for i := range e.Sources {
		i := i
		e.Controls = append(e.Controls, forms.Control{Label: fmt.Sprintf("Source %d", i+1), Text: &e.Sources[i], Help: "Paths/globs relative to the main config. Order determines source precedence, never overwrites."}, forms.Control{Label: "Move source up", Action: func() {
			if i > 0 {
				e.Sources[i-1], e.Sources[i] = e.Sources[i], e.Sources[i-1]
			}
		}}, forms.Control{Label: "Move source down", Action: func() {
			if i+1 < len(e.Sources) {
				e.Sources[i+1], e.Sources[i] = e.Sources[i], e.Sources[i+1]
			}
		}}, forms.Control{Label: "Remove source (does not delete the file)", Action: func() { e.Sources = append(e.Sources[:i], e.Sources[i+1:]...) }})
	}
	e.Controls = append(e.Controls, forms.Control{Label: "Add source", Action: func() { e.Sources = append(e.Sources, text("")) }}, forms.Control{Label: "Discover current-directory .csnippets", Toggle: &e.Project}, forms.Control{Label: "Default destination", Text: &e.Default, Help: "Empty means main config. New files must match an included source pattern."}, forms.Control{Label: "Color", Selection: &e.Color, Options: []string{"auto", "always", "never"}})
	e.Focus = max(0, min(e.Focus, len(e.Controls)-1))
}
func (e *SettingsEditor) Update(key tea.KeyMsg) {
	switch key.String() {
	case "tab":
		e.Focus = (e.Focus + 1) % len(e.Controls)
	case "shift+tab":
		e.Focus = (e.Focus + len(e.Controls) - 1) % len(e.Controls)
	case "enter":
		if e.Controls[e.Focus].Action != nil {
			e.Controls[e.Focus].Action()
		} else {
			e.Focus = (e.Focus + 1) % len(e.Controls)
		}
	default:
		e.Controls[e.Focus].Update(key)
		e.Error = ""
	}
	e.Build()
}
func (e *SettingsEditor) View(width, height int) string {
	dirty := "clean"
	if e.Dirty() {
		dirty = "unsaved"
	}
	header := forms.Fit("Library settings ["+dirty+"]", width, 1, 0)
	footer := forms.Fit("Ctrl-S:save Esc:back\nTab:next F1:help", width, height, 0)
	footerRows := strings.Count(footer, "\n") + 1
	errorRows := 0
	if e.Error != "" {
		errorRows = 1
	}
	view := header + "\n" + forms.ControlsView(e.Controls, e.Focus, width, max(1, height-1-footerRows-errorRows)) + "\n"
	if errorRows > 0 {
		view += forms.Fit(forms.Safe(e.Error), width, 1, 0) + "\n"
	}
	return view + footer
}
