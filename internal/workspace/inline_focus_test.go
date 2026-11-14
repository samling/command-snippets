package workspace

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/models"
	"testing"
)

func TestEditorChoiceRowsKeepTraversalIdentity(t *testing.T) {
	m, _ := New(fixtureLibrary(t, "snippets: []\n"), Options{Start: "add", NoColor: true})
	e := m.Editor
	e.Inputs = append(e.Inputs, newInput(models.Input{Name: "sort_by", Kind: "choice", Choices: []models.Choice{{Label: "CPU", Value: "3"}}}))
	e.openInput(e.Inputs[0])
	e.Build()
	for i, c := range e.Controls {
		if c.Label == "+ Add choice" {
			e.Focus = i
		}
	}
	press(m, tea.KeyEnter) // + Add choice focuses the new choice directly
	if e.Controls[e.Focus].Text != &e.Inputs[0].Choices[1].Key {
		t.Fatalf("new choice traversal returned to old row: focus=%d label=%s", e.Focus, e.Controls[e.Focus].Label)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Memory")})
	press(m, tea.KeyTab)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	draft, _ := e.Draft()
	if draft.Inputs[0].Choices[0].Label != "CPU" || draft.Inputs[0].Choices[1].Label != "Memory" || draft.Inputs[0].Choices[1].Value != "4" {
		t.Fatalf("choice typing changed another row: %+v", draft.Inputs[0].Choices)
	}
}
