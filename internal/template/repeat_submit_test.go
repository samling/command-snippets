package template

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/models"
)

func TestRepeatFormHasExplicitEnterSubmission(t *testing.T) {
	SetupColorProfile(true)
	f, err := NewForm(models.Snippet{Name: "Repeat", Command: "echo {{items}}", Inputs: []models.Input{{Name: "items", Kind: "repeat", Flag: "-i", Default: []string{"one"}, Required: true}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	last := len(f.Controls) - 1
	if !f.Controls[last].Submit || !strings.Contains(f.Controls[last].View(true), "[Enter]") {
		t.Fatal("repeat form has no explicit Enter submission row")
	}
	f.Focus = last
	if !f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || f.Result.Command != "echo -i one" {
		t.Fatal("Enter did not submit a valid repeat form")
	}
	f.Repeats["items"] = nil
	f.Refresh()
	if f.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("Enter submitted an empty required repeat")
	}
	f.Focus = 0 // Add item stays an action, not submission.
	if !strings.Contains(f.View(90, 24), "Enter:activate") || f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || len(f.Repeats["items"]) != 1 {
		t.Fatal("Add item action was replaced by submission")
	}
	f.Repeats["items"][0].Set("two words")
	f.Refresh()
	f.Focus = len(f.Controls) - 1
	if !strings.Contains(f.View(90, 24), "Enter:submit") || !f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || f.Result.Command != "echo -i 'two words'" {
		t.Fatal("Enter did not insert a quoted repeat item")
	}
	f.Focus = 1 // Remove item stays an action, not submission.
	if f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || len(f.Repeats["items"]) != 0 {
		t.Fatal("Remove item action was replaced by submission")
	}
	f.Focus = 0
	f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	f.Focus = 0
	f.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if len(f.Repeats["items"]) != 0 {
		t.Fatal("Ctrl-D repeat removal was intercepted by regex scrolling")
	}
}
