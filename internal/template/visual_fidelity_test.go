package template

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/samling/command-snippets/internal/models"
	"strings"
	"testing"
)

func TestPartialLivePreviewKeepsFilledValuesAndUnfilledPlaceholders(t *testing.T) {
	profile := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(profile)
	lipgloss.SetColorProfile(termenv.TrueColor)
	f, err := NewForm(models.Snippet{Name: "Partial", Command: "echo {{first}} {{second}}", Inputs: []models.Input{{Name: "first", Default: "two words"}, {Name: "second", Required: true}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	view := f.View(120, 30)
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "Template: Partial") || !strings.Contains(plain, "Inputs") || !strings.Contains(plain, "Live preview") || !strings.Contains(plain, "echo 'two words' ‹second›") || !strings.Contains(plain, "required") || strings.Contains(plain, "is required") || strings.Contains(plain, "invalid — complete") {
		t.Fatalf("partial preview/layout missing: %s", plain)
	}
	if !strings.Contains(view, lipgloss.NewStyle().Foreground(lipgloss.Color("120")).Underline(true).Render("'two words'")) || !strings.Contains(view, lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Render("‹second›")) {
		t.Fatalf("separate preview accents missing: %q", view)
	}
	if f.Result.Command != "" || f.Update(tea.KeyMsg{Type: tea.KeyCtrlS}) {
		t.Fatal("partial preview became executable")
	}
	f.Texts["second"].Set("done")
	f.Refresh()
	if !strings.Contains(ansi.Strip(f.View(120, 30)), "echo 'two words' done") {
		t.Fatal("complete preview missing")
	}
}

func TestExpressionPortPreviewDoesNotWaitForRequiredImage(t *testing.T) {
	profile := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(profile)
	SetupColorProfile(true)
	f, err := NewForm(models.Snippet{
		Name: "Docker run", Command: "docker run {{port_arg}} {{image_name_arg}}",
		Inputs: []models.Input{
			{Name: "port", Validate: &models.Validation{Range: []int{1, 65535}}},
			{Name: "image_name", Required: true},
		},
		Expressions: map[string]string{
			"port_arg":       `inputs.port == "" ? "" : flag("-p", inputs.port + ":" + inputs.port)`,
			"image_name_arg": `inputs.image_name`,
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Texts["port"].Set("8080")
	f.Refresh()
	if !strings.Contains(f.View(120, 30), "docker run -p 8080:8080 ‹image_name›") {
		t.Fatalf("valid optional expression missing from live preview: %s", f.View(120, 30))
	}
	if f.Update(tea.KeyMsg{Type: tea.KeyCtrlS}) || f.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("incomplete expression preview submitted")
	}
	if f.Focus != 1 || f.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("Enter did not advance to and block the required image input")
	}
	f.Texts["image_name"].Set("alpine")
	f.Refresh()
	if !f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || f.Result.Command != "docker run -p 8080:8080 alpine" {
		t.Fatal("completed expression form did not submit with Enter")
	}
}

func TestRegexExplanationOnlyBesideFocusedRegex(t *testing.T) {
	SetupColorProfile(true)
	f, err := NewForm(models.Snippet{Name: "Regex", Command: "grep {{pattern}} {{file}}", Inputs: []models.Input{{Name: "pattern", Default: "^foo.*$", Validate: &models.Validation{Regex: true}}, {Name: "file", Default: "file.txt"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wide := f.View(120, 30)
	if !strings.Contains(wide, "Pattern Explanation") || !strings.Contains(wide, "PgUp/PgDn") {
		t.Fatalf("regex pane/shortcuts absent: %s", wide)
	}
	for _, line := range strings.Split(wide, "\n") {
		if strings.Contains(line, "Pattern Explanation") && ansi.StringWidth(line[:strings.Index(line, "Pattern Explanation")]) < 120/2 {
			t.Fatalf("pane below rather than beside form: %s", line)
		}
	}
	for _, size := range [][2]int{{120, 12}, {120, 10}} {
		view := f.View(size[0], size[1])
		assertFormBounds(t, view, size)
		if !strings.Contains(view, "Pattern Explanation") || !strings.Contains(view, "|") || !strings.Contains(view, "Esc:back") {
			t.Fatalf("short wide regex pane/cursor/footer missing at %v: %s", size, view)
		}
	}
	f.Focus = 1
	if strings.Contains(f.View(120, 30), "Pattern Explanation") {
		t.Fatal("unfocused regex pane shown")
	}
	f.Focus = 0
	if strings.Contains(f.View(90, 24), "Pattern Explanation") {
		t.Fatal("regex pane shown when narrow")
	}
	f.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if strings.Contains(f.View(120, 30), "Pattern Explanation") || !strings.Contains(f.View(120, 30), "Ctrl-R:pane") {
		t.Fatal("pane toggle or recovery shortcut lost")
	}
	f.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if !strings.Contains(f.View(120, 30), "Pattern Explanation") {
		t.Fatal("pane did not reopen")
	}
	f.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if f.RegexScroll == 0 {
		t.Fatal("regex scroll lost")
	}
}

func TestValidationFailureVisibleInTinyForm(t *testing.T) {
	SetupColorProfile(true)
	f, err := NewForm(models.Snippet{Name: "Required", Command: "echo {{value}}", Inputs: []models.Input{{Name: "value", Required: true}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	view := f.View(24, 6)
	if !strings.Contains(view, "required") || !strings.Contains(view, "Esc:back") || f.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatalf("tiny validation failure hidden or submitted: %s", view)
	}
}

func TestChoicesUseSpacedOriginalLayout(t *testing.T) {
	SetupColorProfile(true)
	value := "one"
	view := Control{Label: "Choice", Selection: &value, Options: []string{"one", "two"}}.View(true)
	if strings.Contains(view, " | ") || !strings.Contains(view, "<one>  two ") {
		t.Fatalf("choices not horizontally spaced: %s", view)
	}
}
