package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/samling/command-snippets/internal/template"
)

// Style definitions for the selector
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("247"))

	scrollStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	helpTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// selectorModel represents a snippet selector
type selectorModel struct {
	options    []string
	snippetMap map[string]string // maps display name to snippet name
	cursor     int
	selected   string
	cancelled  bool
	done       bool
}

// newSelectorModel creates a new selector model from prebuilt display options.
func newSelectorModel(options []string, snippetMap map[string]string) selectorModel {
	return selectorModel{
		options:    options,
		snippetMap: snippetMap,
	}
}

// Init initializes the model
func (m selectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}

		case "enter":
			m.selected = m.snippetMap[m.options[m.cursor]]
			m.done = true
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the selector
func (m selectorModel) View() string {
	if m.done || m.cancelled {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Select a template to execute:"))
	b.WriteString("\n\n")

	// Show visible options (window of items around cursor)
	windowSize := 10
	start := m.cursor - windowSize/2
	if start < 0 {
		start = 0
	}
	end := start + windowSize
	if end > len(m.options) {
		end = len(m.options)
		start = end - windowSize
		if start < 0 {
			start = 0
		}
	}

	// Show scroll indicator if needed
	if start > 0 {
		b.WriteString(scrollStyle.Render("  ...\n"))
	}

	for i := start; i < end; i++ {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + m.options[i]))
		} else {
			b.WriteString(normalStyle.Render("  " + m.options[i]))
		}
		b.WriteString("\n")
	}

	// Show scroll indicator if needed
	if end < len(m.options) {
		b.WriteString(scrollStyle.Render("  ...\n"))
	}

	b.WriteString("\n")
	b.WriteString(helpTextStyle.Render("↑/k: Up  ↓/j: Down  Enter: Select  q/Esc: Cancel"))

	return b.String()
}

// selectSnippetWithBubbleTea shows an interactive snippet selector using Bubble Tea
func selectSnippetWithBubbleTea(options []string, snippetMap map[string]string, noColor bool) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no templates found")
	}

	template.SetupColorProfile(noColor)

	model := newSelectorModel(options, snippetMap)
	p := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	selector := finalModel.(selectorModel)
	if selector.cancelled {
		return "", &UserCancellationError{"user cancelled selection"}
	}
	return selector.selected, nil
}
