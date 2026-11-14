package workspace

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestCompactLibraryShowsRenderFailureAndCancellation(t *testing.T) {
	lib := fixtureLibrary(t, "snippets:\n  - name: Invalid command\n    command: echo {{bad}}\n    expressions: {bad: '\"\\x00\"'}\n")
	if err := lib.Error(); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{40, 10}, {24, 6}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m, err := New(lib, Options{NoColor: true})
			if err != nil {
				t.Fatal(err)
			}
			m.Width, m.Height = size[0], size[1]
			press(m, tea.KeyEnter)
			view := m.View()
			plain := strings.Join(strings.Fields(view), " ")
			if m.Done || m.Command != "" || !strings.Contains(plain, "NUL cannot appear in a shell command") || !strings.Contains(view, "Esc:cancel") {
				t.Fatalf("render failure hidden or emitted at %v: %s", size, view)
			}
			assertReviewDimensions(t, view, size)
		})
	}
}

func TestNarrowDetailToggleKeepsFocusedCategoriesVisible(t *testing.T) {
	m, err := New(fixtureLibrary(t, "snippets:\n  - name: List pods\n    tags: [Kubernetes]\n    command: kubectl get pods\n"), Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 60, 18
	press(m, tea.KeyCtrlD)
	press(m, tea.KeyTab)
	if m.Focus != 1 || !strings.Contains(m.View(), "Categories") || strings.Contains(m.View(), "Command Preview") {
		t.Fatalf("focused categories hidden behind detail: %s", m.View())
	}
	press(m, tea.KeyDown)
	press(m, tea.KeyDown)
	press(m, tea.KeyEnter)
	if len(m.Tags) != 1 || !strings.Contains(m.View(), "Kubernetes") {
		t.Fatalf("visible category selection lost: %s", m.View())
	}
}

func TestNarrowLibrarySearchKeepsActiveFilterVisible(t *testing.T) {
	m, err := New(fixtureLibrary(t, "snippets:\n  - name: List pods\n    tags: [Kubernetes]\n    command: kubectl get pods\n"), Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Category = 2
	m.toggleCategory()
	m.Query.Set("pods")
	m.refresh("")
	for _, size := range [][2]int{{40, 16}, {40, 10}, {24, 6}} {
		m.Width, m.Height = size[0], size[1]
		view := m.View()
		filterVisible := false
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, "Tags: Kubernetes") || strings.Contains(line, "tag: Kubernetes") {
				filterVisible = true
			}
		}
		if !filterVisible {
			t.Fatalf("active filter hidden at %v: %s", size, view)
		}
		assertReviewDimensions(t, view, size)
	}
}

func TestCategorySelectionBackgroundFillsRow(t *testing.T) {
	profile := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(profile)
	m, err := New(fixtureLibrary(t, "settings:\n  color: always\nsnippets:\n  - name: List pods\n    tags: [Kubernetes]\n    command: kubectl get pods\n"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height, m.Category = 120, 30, 2
	m.toggleCategory()
	for _, focus := range []int{1, 2} {
		m.Focus = focus
		style := m.Styles.InactiveRow
		if focus == 1 {
			style = m.Styles.ActiveRow
		}
		want := style.Width(m.Width/5 - 4).Render("✓ Kubernetes (1)")
		if !strings.Contains(m.View(), want) {
			t.Fatalf("selection background does not fill category row at focus %d", focus)
		}
	}
}

func TestTinyCategoryFocusRemainsVisible(t *testing.T) {
	m, err := New(fixtureLibrary(t, "snippets:\n  - name: List pods\n    tags: [Kubernetes]\n    command: kubectl get pods\n"), Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 24, 6
	press(m, tea.KeyTab)
	press(m, tea.KeyDown)
	press(m, tea.KeyDown)
	press(m, tea.KeyEnter)
	view := m.View()
	if !strings.Contains(view, "> ✓ Kubernetes") || !strings.Contains(view, "Tags: Kubernetes") || !strings.Contains(view, "Esc:cancel") {
		t.Fatalf("focused category is outside compact viewport: %s", view)
	}
	assertReviewDimensions(t, view, [2]int{24, 6})
}

func assertReviewDimensions(t *testing.T, view string, size [2]int) {
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
