package template

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/theme"
)

func TestTransparentMochaStylesKeepForegroundsAndSelection(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	settings := models.Settings{Color: "always", Theme: "catppuccin-mocha", ThemeColors: map[string]string{"background": "transparent", "panel": "transparent"}}
	s := NewStyles(settings, false)
	if s.Colors[theme.Background] != "" || s.Colors[theme.Panel] != "" {
		t.Fatal("transparent token reached styles")
	}
	view := s.Surface(s.Frame("panel", 30, 6, false), 40, 10)
	if strings.Contains(view, "48;") || !strings.Contains(view, s.Text.Render("panel")) {
		t.Fatalf("surface background or missing Mocha text: %q", view)
	}
	if title := s.Surface("unstyled title", 40, 10); strings.Contains(title, "48;") || !strings.Contains(title, s.Text.Render("unstyled title")) {
		t.Fatalf("transparent Mocha lost text foreground on unstyled headers: %q", title)
	}
	if s.Selected.GetBackground() != lipgloss.Color("#45475a") || !strings.Contains(s.Selected.Render("selected"), "48;2;") {
		t.Fatal("transparency removed selection highlight")
	}
	settings.Color = "never"
	if plain := NewStyles(settings, false).Surface("plain", 30, 6); strings.Contains(plain, "\x1b") {
		t.Fatal("no-color transparent surface emitted ANSI")
	}
}

func TestOriginalFormPaletteAndRuneBlockCursor(t *testing.T) {
	profile := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(profile)
	lipgloss.SetColorProfile(termenv.TrueColor)
	text := NewText("a日本", false)
	text.Cursor = 1
	control := Control{Label: "Message", Help: "Description", Text: &text}
	view := control.View(true)
	if !strings.Contains(view, "\x1b[") || !strings.Contains(view, "\x1b[7m日") || ansi.Strip(view) != "> Message (Description): a日本" {
		t.Fatalf("original focus palette/rune block cursor missing: %q", view)
	}
	other := "one"
	choice := Control{Label: "Choice", Selection: &other, Options: []string{"one", "two"}}
	if !strings.Contains(ansi.Strip(choice.View(true)), "<one>") {
		t.Fatal("original selected-choice presentation lost")
	}
	SetupColorProfile(true)
	view = control.View(true)
	if strings.Contains(view, "\x1b[") || !strings.Contains(view, "a|日本") {
		t.Fatal("plain terminal focus/cursor fallback missing")
	}
}

func TestInputFormRestoresOriginalPaletteWithoutChangingSharedControls(t *testing.T) {
	profile := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(profile)
	lipgloss.SetColorProfile(termenv.TrueColor)
	text := NewText("shared", false)
	shared := Control{Label: "Shared", Text: &text}
	before := shared.View(true)
	f, err := NewForm(models.Snippet{
		Name: "Original palette", Command: "echo {{value}} {{missing}} {{choice}}",
		Inputs: []models.Input{
			{Name: "value", Label: "Value", Default: "filled"},
			{Name: "missing", Label: "Missing", Required: true},
			{Name: "choice", Label: "Choice", Kind: "choice", Default: "One", Choices: []models.Choice{{Label: "One", Value: "1"}, {Label: "Two", Value: "2"}}},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	view := f.View(120, 30)
	for _, tc := range []struct {
		text, color string
		bold        bool
	}{
		{"Value", "205", false},
		{"Missing", "241", false},
		{"Live preview", "86", true},
		{"echo ", "86", false},
		{"‹Missing›", "208", true},
		{"Live preview", "86", true},
		{"<One>", "86", true},
		{" Two ", "247", false},
		{"required", "241", false},
	} {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(tc.color)).Bold(tc.bold)
		if !strings.Contains(view, style.Render(tc.text)) {
			t.Fatalf("original form color %s missing for %q: %q", tc.color, tc.text, view)
		}
	}
	if after := shared.View(true); after != before || !strings.Contains(after, FocusedStyle.Render("Shared:")) {
		t.Fatal("rendering the original form palette changed the shared muted theme")
	}
	SetupColorProfile(true)
	plain := f.View(120, 30)
	if ansi.Strip(plain) != plain || !strings.Contains(plain, "Live preview") || !strings.Contains(plain, "Esc:back Ctrl-C:cancel") {
		t.Fatal("original form palette broke no-color rendering")
	}
}
