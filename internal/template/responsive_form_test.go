package template

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/theme"
	"regexp"
)

func TestResponsiveFormPanelsAndHierarchy(t *testing.T) {
	f, err := NewForm(models.Snippet{Name: "Panels", Command: "echo {{first}} {{second}}", Inputs: []models.Input{{Name: "first", Default: "filled", Help: "First description"}, {Name: "second", Required: true, Help: "Second description"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Styles = NewStyles(models.Settings{Color: "never"}, true)
	for _, size := range [][2]int{{120, 30}, {90, 24}, {60, 18}} {
		view := f.View(size[0], size[1])
		assertFormBounds(t, view, size)
		inputRow, previewRow := -1, -1
		for i, line := range strings.Split(view, "\n") {
			if strings.Contains(line, "Inputs") {
				inputRow = i
			}
			if strings.Contains(line, "Live preview") {
				previewRow = i
			}
		}
		if inputRow < 0 || previewRow < 0 || !strings.Contains(view, "╭") || !strings.Contains(view, "Insert command") {
			t.Fatalf("panels/action missing at %v: %s", size, view)
		}
		if (inputRow == previewRow) != (size[0] >= 90) {
			t.Fatalf("wrong responsive orientation at %v: %s", size, view)
		}
		if strings.Contains(view, "(First description)") || strings.Contains(view, "invalid — complete") {
			t.Fatalf("inline help/generic error obscures fields: %s", view)
		}
		// Descriptions belong to the focused field's hint; errors stay explainable.
		if size[1] >= 24 && (!strings.Contains(view, "First description") || strings.Contains(view, "Second description") || !strings.Contains(view, "required") || strings.Contains(view, "is required")) {
			t.Fatalf("local help/error missing: %s", view)
		}
	}
}

func TestResponsiveFormCursorAndCompactDiagnostics(t *testing.T) {
	f, err := NewForm(models.Snippet{Name: "Cursor", Command: "echo {{value}} {{required}}", Inputs: []models.Input{{Name: "value", Default: strings.Repeat("a", 150) + "日"}, {Name: "required", Required: true}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Styles = NewStyles(models.Settings{Color: "never"}, true)
	for _, size := range [][2]int{{120, 30}, {60, 18}, {40, 10}, {24, 6}, {20, 5}} {
		for _, focus := range []int{0, 1} {
			f.Focus = focus
			view := f.View(size[0], size[1])
			assertFormBounds(t, view, size)
			// 20x5 is below the workspace minimum; there is only room for the cursor and footer.
			if !strings.Contains(view, "|") || !strings.Contains(view, "Esc:back") || (size[0] >= 24 && !strings.Contains(view, "required")) || strings.Contains(view, "is required") {
				t.Fatalf("cursor/blocked reason/footer lost at %v focus %d: %s", size, focus, view)
			}
			if strings.Contains(view, "\x1b") {
				t.Fatal("no-color emitted ANSI")
			}
		}
	}
}

func TestResponsiveFormNoInputsAndRepeatFollowingField(t *testing.T) {
	f, err := NewForm(models.Snippet{Name: "Static", Command: "echo ready"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Styles = NewStyles(models.Settings{Color: "never"}, true)
	for _, size := range [][2]int{{120, 30}, {60, 18}, {40, 10}, {24, 6}, {20, 5}} {
		view := f.View(size[0], size[1])
		assertFormBounds(t, view, size)
		if !strings.Contains(view, "Insert command") || !strings.Contains(view, "Enter") || !strings.Contains(view, "Esc:back") || !f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || f.Result.Command != "echo ready" {
			t.Fatalf("no-input Enter insertion missing at %v: %s", size, view)
		}
	}
	f, err = NewForm(models.Snippet{Name: "Repeat", Command: "echo {{items}} {{after}}", Inputs: []models.Input{{Name: "items", Kind: "repeat", Flag: "-i", Default: []string{"one"}}, {Name: "after", Default: "tail"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Styles = NewStyles(models.Settings{Color: "never"}, true)
	for _, id := range []string{"items", "items-remove-0", "after"} {
		for i, c := range f.Controls {
			if c.ID == id {
				f.Focus = i
			}
		}
		view := f.View(120, 30)
		if !strings.Contains(view, "Add items item") || !strings.Contains(view, "Remove item") || !strings.Contains(view, "after  ") {
			t.Fatalf("repeat/following controls missing: %s", view)
		}
	}
	f.Focus = 2
	if f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || len(f.Repeats["items"]) != 2 {
		t.Fatal("Add stopped working")
	}
	f.Focus = 1
	if f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || len(f.Repeats["items"]) != 1 {
		t.Fatal("Remove stopped working")
	}
	f.Repeats["items"][0].Set("two")
	f.Refresh()
	f.Focus = len(f.Controls) - 1
	if !f.Update(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("following field cannot submit")
	}
}

func TestResponsiveFormColoredCursorCardOffsets(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	f, err := NewForm(models.Snippet{Name: "Cursor offsets", Command: "echo {{value}} {{choice}}", Inputs: []models.Input{{Name: "value", Help: "Keep the cursor reachable"}, {Name: "choice", Kind: "choice", Default: "two", Choices: []models.Choice{{Label: "one", Value: "1"}, {Label: "two", Value: "2"}}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Styles = NewStyles(models.Settings{Color: "always", Theme: "catppuccin-mocha", ThemeColors: map[string]string{"background": "transparent", "panel": "transparent"}}, false)
	for _, size := range [][2]int{{120, 30}, {120, 14}, {60, 18}, {40, 10}, {24, 6}, {20, 5}} {
		for _, position := range []int{0, 13, 49, 50, 51, 105, 149} {
			f.Texts["value"].Set(strings.Repeat("a", position) + "日" + strings.Repeat("z", 200))
			f.Texts["value"].Cursor = position
			f.Refresh()
			view := f.View(size[0], size[1])
			assertFormBounds(t, view, size)
			if !strings.Contains(view, "\x1b[7m日") || !strings.Contains(ansi.Strip(view), "value") {
				t.Fatalf("card offset lost focused field/wide cursor at %v position %d: %q", size, position, view)
			}
		}
		f.Focus = 1
		view := f.View(size[0], size[1])
		if !strings.Contains(ansi.Strip(view), "<two>") {
			t.Fatalf("focused choice hidden at %v: %s", size, view)
		}
		f.Focus = 0
	}
}

func assertFormBounds(t *testing.T, view string, size [2]int) {
	t.Helper()
	if strings.Count(view, "\n")+1 > size[1] {
		t.Fatalf("height exceeded %v: %s", size, view)
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > size[0] {
			t.Fatalf("width exceeded %v: %s", size, line)
		}
	}
}

func TestResponsiveFormShortStackedSelectionValueVisible(t *testing.T) {
	for _, kind := range []string{"choice", "toggle"} {
		f, err := NewForm(models.Snippet{Name: "Short stacked", Command: "echo {{first}} {{second}}", Inputs: []models.Input{{Name: "first", Default: "hello"}, {Name: "second", Kind: kind, Default: map[string]any{"choice": "two", "toggle": true}[kind], Flag: map[string]string{"toggle": "-s"}[kind], Choices: map[string][]models.Choice{"choice": {{Label: "one", Value: "1"}, {Label: "two", Value: "2"}}}[kind]}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		f.Styles = NewStyles(models.Settings{Color: "never"}, true)
		f.Focus = 1
		want := map[string]string{"choice": "<two>", "toggle": "[on]"}[kind]
		view := ansi.Strip(f.View(60, 14))
		if !strings.Contains(view, want) {
			t.Errorf("%s focused value missing: %s", kind, view)
		}
	}
}

func TestResponsiveFormWhitespaceCursorVisible(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for _, plain := range []bool{true, false} {
		f, err := NewForm(models.Snippet{Name: "Word wrap", Command: "echo {{value}}", Inputs: []models.Input{{Name: "value", Default: strings.Repeat("a", 50) + " xyz"}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		f.Styles = NewStyles(models.Settings{Color: "always", Theme: "catppuccin-mocha", ThemeColors: map[string]string{"background": "transparent", "panel": "transparent"}}, plain)
		f.Texts["value"].Cursor = 51
		view := f.View(60, 14)
		want := "|xyz"
		if !plain {
			want = "\x1b[7mx"
		}
		if !strings.Contains(view, want) {
			t.Errorf("plain=%v cursor-containing word hidden: %q", plain, view)
		}
	}
}

func TestResponsiveFormWrappedToggleMarkerKeepsValue(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for _, plain := range []bool{true, false} {
		s := NewStyles(models.Settings{Color: "always", Theme: "catppuccin-mocha"}, plain)
		value := true
		view, _ := alignedView([]Control{{Label: strings.Repeat("a", 55) + "[on]", Toggle: &value, Help: "HELP [on]"}}, 0, 56, 1, s.InputControls)
		if text := strings.TrimRight(ansi.Strip(view), " "); !strings.HasSuffix(text, " [on]") || strings.Count(text, "[on]") != 1 || strings.Contains(text, "HELP") {
			t.Errorf("plain=%v focused toggle redirected into label/help: %q", plain, view)
		}
	}
}

func alignedForm(t *testing.T, color bool) *Form {
	t.Helper()
	f, err := NewForm(models.Snippet{Name: "Docker", Command: "docker run {{port}} {{image}} {{format}} {{sign}}", Inputs: []models.Input{
		{Name: "port", Label: "Port", Help: "Optional port mapping", Default: "8080"},
		{Name: "image", Label: "Image", Help: "Required Docker image", Required: true},
		{Name: "format", Label: "Format", Kind: "choice", Default: "wide", Choices: []models.Choice{{Label: "wide", Value: "-o wide"}, {Label: "json", Value: "-o json"}, {Label: "yaml", Value: "-o yaml"}}},
		{Name: "sign", Label: "Sign", Kind: "toggle", Flag: "--sign", Default: false},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Styles = NewStyles(models.Settings{Color: "never"}, true)
	if color {
		f.Styles = NewStyles(models.Settings{Color: "always", Theme: "catppuccin-mocha"}, false)
	}
	return f
}

func TestAlignedFormRowsAndFocusHint(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for _, color := range []bool{false, true} {
		for _, size := range [][2]int{{120, 30}, {70, 30}} {
			f := alignedForm(t, color)
			view := f.View(size[0], size[1])
			assertFormBounds(t, view, size)
			plain := ansi.Strip(view)
			columns := map[string]int{}
			focusMarker := "> Port "
			if color {
				focusMarker = "▌ Port "
			}
			for _, line := range strings.Split(strings.ReplaceAll(plain, focusMarker, "> Port "), "\n") {
				for label, value := range map[string]string{"> Port ": "8080", "  Image ": "required", "  Format ": "<wide>", "  Sign ": "[off]"} {
					if index := strings.Index(line, label); index >= 0 && strings.Contains(line, value) {
						columns[label] = strings.Index(line, value) - index
					}
				}
			}
			if len(columns) != 4 || columns["> Port "] != columns["  Image "] || columns["> Port "] != columns["  Format "] || columns["> Port "] != columns["  Sign "] {
				t.Fatalf("color=%v %v: rows are not aligned %v:\n%s", color, size, columns, plain)
			}
			if !strings.Contains(plain, "Optional port mapping") || strings.Contains(plain, "Required Docker image") || strings.Contains(plain, "Port:") {
				t.Fatalf("color=%v %v: hint/description layout wrong:\n%s", color, size, plain)
			}
			// An empty required field is marked once, in place; the preview names it.
			if strings.Contains(plain, "is required") || strings.Contains(plain, "Image *") || strings.Contains(plain, " !") || !strings.Contains(plain, "‹Image›") {
				t.Fatalf("color=%v %v: required shown noisily or preview unnamed:\n%s", color, size, plain)
			}
			f.Update(tea.KeyMsg{Type: tea.KeyDown})
			view = f.View(size[0], size[1])
			plain = ansi.Strip(view)
			if !strings.Contains(plain, "Required Docker image") || strings.Contains(plain, "is required") || strings.Contains(plain, "Optional port mapping") {
				t.Fatalf("color=%v %v: focused Image hint wrong:\n%s", color, size, plain)
			}
			if !color && strings.Contains(view, "\x1b") {
				t.Fatal("no-color form emitted ANSI")
			}
		}
	}
}

func TestStackedLivePreviewKeepsCommandWithManyErrors(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for _, plain := range []bool{true, false} {
		f, err := NewForm(models.Snippet{Name: "Many", Command: "run {{a}} {{b}} {{c}} {{d}}", Inputs: []models.Input{{Name: "a", Required: true}, {Name: "b", Required: true}, {Name: "c", Required: true}, {Name: "d", Required: true}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		f.Styles = NewStyles(models.Settings{Color: "always", Theme: "catppuccin-mocha", ThemeColors: map[string]string{"background": "transparent", "panel": "transparent"}}, plain)
		for _, size := range [][2]int{{80, 24}, {70, 30}, {120, 30}} {
			view := ansi.Strip(f.View(size[0], size[1]))
			if !strings.Contains(view, "run ‹a› ‹b› ‹c› ‹d›") || strings.Contains(view, "is required") {
				t.Errorf("plain=%v %v partial preview or grouped blocking reason missing:\n%s", plain, size, view)
			}
		}
	}
}

func logsForm(t *testing.T, plain bool) *Form {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	f, err := NewForm(models.Snippet{Name: "Analyze Logs with Regex", Command: "awk {{awk_script}} {{logfile_arg}} | sed -E {{sed_script}}", Inputs: []models.Input{
		{Name: "pattern", Help: "Initial filter pattern (must be valid regex)", Required: true, Validate: &models.Validation{Regex: true}},
		{Name: "extract_pattern", Help: "Pattern to extract specific parts", Default: "(.*)", Validate: &models.Validation{Regex: true}},
		{Name: "logfile", Help: "Log file to analyze", Required: true},
	}, Expressions: map[string]string{"awk_script": `quote("/" + inputs.pattern + "/ {print $0}")`, "logfile_arg": "inputs.logfile", "sed_script": `quote("s/" + inputs.extract_pattern + "/\\1/g")`}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Styles = NewStyles(models.Settings{Color: "always", Theme: "catppuccin-mocha", ThemeColors: map[string]string{"background": "transparent", "panel": "transparent"}}, plain)
	return f
}

func TestFormPreviewNamesMissingInputsAndLinksFocus(t *testing.T) {
	f := logsForm(t, true)
	for _, size := range [][2]int{{120, 24}, {80, 24}} {
		plain := f.View(size[0], size[1])
		if !strings.Contains(plain, "awk ‹pattern› ‹logfile› | sed -E 's/(.*)/\\1/g'") || strings.Contains(plain, "{{awk_script}}") || strings.Contains(plain, "{{logfile_arg}}") {
			t.Fatalf("%v preview does not name missing inputs:\n%s", size, plain)
		}
	}
	color := logsForm(t, false)
	link := color.Styles.InputUnfilled.Underline(true).Render("‹pattern›")
	if view := color.View(120, 24); !strings.Contains(view, link) || strings.Contains(view, color.Styles.InputUnfilled.Underline(true).Render("‹logfile›")) {
		t.Fatalf("focused input's preview part not linked: %q", view)
	}
	color.Texts["pattern"].Set("error")
	color.Refresh()
	if view := color.View(120, 24); !strings.Contains(view, color.Styles.InputFilled.Underline(true).Render("'/error/ {print $0}'")) {
		t.Fatalf("filled focused fragment not linked: %q", view)
	}
}

func TestFormRequiredUsesOneQuietMarker(t *testing.T) {
	for _, plainMode := range []bool{true, false} {
		for _, focus := range []int{0, 2} {
			f := logsForm(t, plainMode)
			f.Focus = focus
			for _, size := range [][2]int{{120, 24}, {80, 24}, {60, 18}, {40, 10}} {
				view := ansi.Strip(f.View(size[0], size[1]))
				if strings.Contains(view, "is required") || strings.Contains(view, "pattern *") || strings.Contains(view, "logfile *") || strings.Contains(view, " !") {
					t.Fatalf("plain=%v focus=%d %v required shown noisily:\n%s", plainMode, focus, size, view)
				}
				if strings.Count(view, "required") < 1 {
					t.Fatalf("plain=%v focus=%d %v required marker missing:\n%s", plainMode, focus, size, view)
				}
			}
		}
	}
	// Real validation errors still explain themselves.
	f := logsForm(t, true)
	f.Texts["extract_pattern"].Set("(")
	f.Refresh()
	f.Focus = 1
	if view := ansi.Strip(f.View(120, 24)); !strings.Contains(view, "invalid regex") {
		t.Fatalf("validation error hidden:\n%s", view)
	}
	c := logsForm(t, false)
	c.Texts["extract_pattern"].Set("(")
	c.Refresh()
	c.Focus = 1
	errPrefix := strings.Split(c.Styles.InputControls.Error.Render("x"), "x")[0]
	if !strings.Contains(c.View(120, 24), errPrefix+"! invalid regex") {
		t.Fatalf("hint error does not use the error role: %q", c.View(120, 24))
	}
}

func TestFormIncompleteInsertJumpsToFirstMissing(t *testing.T) {
	f := logsForm(t, true)
	f.Texts["pattern"].Set("error")
	f.Refresh()
	f.Focus = len(f.Controls) - 1
	if f.Update(tea.KeyMsg{Type: tea.KeyEnter}) || f.Controls[f.Focus].ID != "logfile" {
		t.Fatalf("Enter on incomplete form focused %q", f.Controls[f.Focus].ID)
	}
	f.Texts["pattern"].Set("")
	f.Refresh()
	f.Focus = 1
	if f.Update(tea.KeyMsg{Type: tea.KeyCtrlS}) || f.Controls[f.Focus].ID != "pattern" {
		t.Fatalf("Ctrl-S on incomplete form focused %q", f.Controls[f.Focus].ID)
	}
	f.Texts["pattern"].Set("error")
	f.Texts["logfile"].Set("app.log")
	f.Refresh()
	if !f.Update(tea.KeyMsg{Type: tea.KeyCtrlS}) || f.Result.Command != "awk '/error/ {print $0}' app.log | sed -E 's/(.*)/\\1/g'" {
		t.Fatalf("complete form did not insert: %q", f.Result.Command)
	}
}

func TestFormInputWellsAndAccentBar(t *testing.T) {
	f := logsForm(t, false)
	view := f.View(120, 24)
	// Compare with what this form's renderer emits for the theme's surfaces.
	sgr := func(color string) string {
		sample := f.Styles.Renderer.NewStyle().Background(lipgloss.Color(color)).Render("x")
		return strings.TrimSuffix(strings.Split(sample, "x")[0], "m")[2:]
	}
	focusBG := sgr(f.Styles.Colors[theme.Selection])
	otherBG := sgr(f.Styles.Colors[theme.InactiveSelection])
	var patternRow, logfileRow string
	for _, line := range strings.Split(view, "\n") {
		plain := ansi.Strip(line)
		if strings.Contains(plain, "pattern") && !strings.Contains(plain, "extract") && patternRow == "" && strings.Contains(plain, "│") {
			patternRow = line
		}
		if strings.Contains(plain, "logfile") && strings.Contains(plain, "required") && !strings.Contains(plain, "‹") {
			logfileRow = line
		}
	}
	if !strings.Contains(ansi.Strip(patternRow), "▌") || !strings.Contains(patternRow, focusBG) {
		t.Fatalf("focused row lacks accent bar or focused well: %q", patternRow)
	}
	if !strings.Contains(logfileRow, otherBG) || strings.Contains(ansi.Strip(logfileRow), "▌") {
		t.Fatalf("unfocused row lacks surface well: %q", logfileRow)
	}
	// Wells are separated by spacing when the panel has room.
	plain := ansi.Strip(view)
	if !regexp.MustCompile(`pattern[^\n]*\n│ +│[^\n]*\n│ +extract_pattern`).MatchString(plain) {
		t.Fatalf("rows are not spaced:\n%s", plain)
	}
	// No-color keeps a textual focus marker and no ANSI.
	nc := logsForm(t, true).View(120, 24)
	if strings.Contains(nc, "\x1b") || !strings.Contains(nc, "> pattern") {
		t.Fatalf("no-color fallback broken:\n%s", nc)
	}
}

func TestRegexPaneKeepsEvenColumns(t *testing.T) {
	f := logsForm(t, true)
	without := ansi.Strip(f.View(120, 24))
	f.Texts["pattern"].Set("err")
	f.Refresh()
	with := ansi.Strip(f.View(120, 24))
	if !strings.Contains(with, "Pattern Explanation") {
		t.Fatalf("regex pane missing:\n%s", with)
	}
	firstBorder := func(view string) int {
		for _, line := range strings.Split(view, "\n") {
			if strings.HasPrefix(line, "╭") {
				return strings.Index(line, "╮")
			}
		}
		return -1
	}
	if a, b := firstBorder(without), firstBorder(with); a < 0 || a != b {
		t.Fatalf("Inputs column width changed with the regex pane: %d vs %d\n%s", a, b, with)
	}
}
