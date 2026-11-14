package template

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/models"
	"testing"
)

func TestFormUsesSharedValidationAndPreview(t *testing.T) {
	s := models.Snippet{Name: "Port", Command: "echo {{port}}", Inputs: []models.Input{{Name: "port", Required: true, Validate: &models.Validation{Range: []int{1, 65535}}}}}
	form, err := NewForm(s, map[string]any{"port": "0"})
	if err != nil {
		t.Fatal(err)
	}
	if form.Result.Valid() || form.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("invalid preset submitted")
	}
	form.Texts["port"].Set("8080")
	form.Refresh()
	if !form.Result.Valid() || form.Result.Command != "echo 8080" || !form.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("valid preview/submit parity failed")
	}
}
func TestTextUnicodePasteAndControlDisplay(t *testing.T) {
	text := NewText("a日本é", true)
	text.Update(tea.KeyMsg{Type: tea.KeyLeft})
	text.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	text.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("世界\nnext"), Paste: true})
	if text.String() != "a日本世界\nnext́" {
		t.Fatalf("rune edit %q", text.String())
	}
	if Safe("\x1b]52;secret\a") != "\\x1b]52;secret\\u0007" {
		t.Fatal("control bytes not escaped")
	}
	for _, width := range []int{0, 1, 20, 40} {
		_ = Fit(text.View(true), width, 5, 0)
	}
}
