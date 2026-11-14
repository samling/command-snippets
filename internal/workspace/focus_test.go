package workspace

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	forms "github.com/samling/command-snippets/internal/template"
)

func TestCommandCursorBelongsOnlyToCommandPane(t *testing.T) {
	m, err := New(fixtureLibrary(t, "snippets:\n  - name: Example\n    command: echo example\n"), Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	for focus := 0; focus < 4; focus++ {
		m.Focus = focus
		for _, height := range []int{1, 10} {
			view := m.results(60, height)
			if strings.Contains(view, "> ") != (focus == 2) {
				t.Fatalf("command cursor shown in wrong pane %d: %q", focus, view)
			}
		}
	}
	m.Focus = 2
	press(m, tea.KeyTab)
	if m.Focus != 3 || strings.Contains(m.results(60, 10), "> ") {
		t.Fatal("command cursor stayed active after leaving its pane")
	}
}

func TestWorkspaceSelectionUsesColorAndPlainFocusFallback(t *testing.T) {
	profile := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(profile)
	m, err := New(fixtureLibrary(t, "settings:\n  color: always\nsnippets:\n  - name: Example\n    tags: [Tools]\n    command: echo example\n"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	m.Focus = 2
	active := m.results(60, 10)
	m.Focus = 1
	inactive := m.results(60, 10)
	if !strings.Contains(active, "\x1b[") || active == inactive || !strings.Contains(m.categories(60), "\x1b[") {
		t.Fatal("active selection/category styling absent")
	}
	plain, err := New(m.Library, Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	plain.Focus = 2
	view := plain.View()
	if ansi.Strip(view) != view || !strings.Contains(view, "> Example") {
		t.Fatal("no-color lost focus or emitted ANSI")
	}
}

func TestPreviewPaneArrowsScrollDetail(t *testing.T) {
	inputs := ""
	for i := range 30 {
		inputs += fmt.Sprintf("      - {name: in%d, help: Input %d description}\n", i, i)
	}
	lib := fixtureLibrary(t, "settings: {color: never}\nsnippets:\n  - name: One\n    command: echo {{in0}}\n    inputs:\n"+inputs+"  - name: Two\n    command: echo two\n")
	m, err := New(lib, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if lib.Error() != nil || m.Mode != "library" {
		t.Fatal(lib.Error())
	}
	m.Width, m.Height, m.Focus = 120, 30, 3
	press(m, tea.KeyDown)
	if m.Selected != 0 || m.DetailScroll != 1 {
		t.Fatalf("Down in preview selected=%d scroll=%d", m.Selected, m.DetailScroll)
	}
	press(m, tea.KeyUp)
	press(m, tea.KeyUp)
	if m.Selected != 0 || m.DetailScroll != 0 {
		t.Fatalf("Up in preview selected=%d scroll=%d", m.Selected, m.DetailScroll)
	}
	_ = m.View()
	for range 100 {
		press(m, tea.KeyDown)
	}
	limit := m.DetailScroll
	press(m, tea.KeyPgDown)
	if m.DetailScroll != limit || limit == 0 || !strings.Contains(m.View(), "Input 29 description") {
		t.Fatalf("preview scroll is not bounded by content: %d then %d\n%s", limit, m.DetailScroll, m.View())
	}
	if !strings.Contains(m.View(), "↑↓/PgUp/PgDn: scroll") {
		t.Fatalf("preview footer omits arrow scrolling: %s", m.View())
	}
	m.Focus = 2
	press(m, tea.KeyDown)
	if m.Selected != 1 || m.DetailScroll != 0 {
		t.Fatal("command pane arrows stopped selecting commands")
	}
}

// assertContinuousBackground fails when visible text after an inner SGR reset
// inside a highlighted span is not preceded by a background sequence.
func assertContinuousBackground(t *testing.T, name, row string) {
	t.Helper()
	parts := strings.Split(row, "\x1b[0m")
	for _, part := range parts[1:] {
		if strings.TrimSpace(ansi.Strip(part)) != "" && strings.TrimSpace(ansi.Strip(part)) != "│" && !strings.Contains(part, "48;") {
			t.Fatalf("%s highlight interrupted at %q in %q", name, part, row)
		}
	}
}

func TestHighlightedRowsKeepBackgroundAcrossStyledSpans(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lib := fixtureLibrary(t, "settings: {color: always, theme: catppuccin-mocha, theme_colors: {background: transparent, panel: transparent}}\nsnippets:\n  - name: Pods\n    tags: [Kubernetes, Monitoring]\n    command: kubectl get pods\n")
	m, err := New(lib, Options{})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 120, 30
	m.Focus = 2
	rows := strings.Split(m.results(50, 20), "\n")
	if len(rows) < 4 || !strings.Contains(rows[2], "48;") || !strings.Contains(rows[3], "48;") {
		t.Fatalf("selected command row lacks a two-line background: %q", rows)
	}
	assertContinuousBackground(t, "command row", rows[2]+rows[3])
	m.Focus, m.Category = 1, 2
	assertContinuousBackground(t, "category", strings.Split(m.categories(30), "\n")[4])
	press(m, tea.KeyF2)
	m.Editor.Name.Set("Folder")
	m.Editor.Command.Set("ls {{folder_name}}")
	m.Editor.Build()
	m.Editor.Focus = 0
	for _, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(ansi.Strip(line), "> Friendly name") {
			row := line[strings.Index(line, "Friendly"):]
			row, _, _ = strings.Cut(row, "│")
			assertContinuousBackground(t, "editor control", row)
		}
	}
	opaque := navigationEditor(t, true)
	opaque.Editor.Focus = 0
	for _, line := range strings.Split(opaque.View(), "\n") {
		if strings.Contains(ansi.Strip(line), "> Friendly name") {
			// The whole line, including the other pane's text, keeps a fill.
			assertContinuousBackground(t, "opaque editor panes", line[strings.Index(line, "Friendly"):])
		}
	}
	m.Mode, m.Editor = "library", nil
	press(m, tea.KeyCtrlO)
	m.Settings.Focus = 0
	m.Settings.Sources = []forms.Text{text("one.yaml")}
	m.Settings.Build()
	for _, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(ansi.Strip(line), "Source 1") {
			assertContinuousBackground(t, "settings control", line)
		}
	}
	plain, err := New(lib, Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	plain.Width, plain.Height, plain.Focus = 120, 30, 2
	if view := plain.View(); strings.Contains(view, "\x1b") || !strings.Contains(view, "> Pods") {
		t.Fatalf("no-color highlight emitted ANSI or lost its marker: %s", view)
	}
}
