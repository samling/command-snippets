package template

import (
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestTextPhysicalSpace(t *testing.T) {
	text := NewText("ls-lah", false)
	text.Cursor = 2
	if !text.Update(tea.KeyMsg{Type: tea.KeySpace}) || text.String() != "ls -lah" || text.Cursor != 3 {
		t.Fatalf("space ignored: %+v", text)
	}
	enabled := false
	control := Control{Toggle: &enabled}
	control.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !enabled {
		t.Fatal("toggle space behavior changed")
	}
}
