package cmd

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/models"
)

// selectorModel represents a snippet selector
type selectorModel struct {
	snippets   map[string]*models.Snippet
	options    []string
	snippetMap map[string]string // maps display name to snippet name
	cursor     int
	selected   string
	cancelled  bool
	done       bool
}

// newSelectorModel creates a new selector model
func newSelectorModel(snippets map[string]*models.Snippet) selectorModel {
	var options []string
	snippetMap := make(map[string]string)

	for name, snippet := range snippets {
		displayName := name
		if snippet.Description != "" {
			displayName = fmt.Sprintf("%s - %s", name, snippet.Description)
		}
		if len(snippet.Tags) > 0 {
			displayName += fmt.Sprintf(" [%s]", strings.Join(snippet.Tags, ", "))
		}

		options = append(options, displayName)
		snippetMap[displayName] = name
	}

	// Sort options alphabetically
	sort.Strings(options)

	return selectorModel{
		snippets:   snippets,
		options:    options,
		snippetMap: snippetMap,
		cursor:     0,
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

	b.WriteString("Select a template to execute:\n\n")

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
		b.WriteString("  ...\n")
	}

	for i := start; i < end; i++ {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, m.options[i]))
	}

	// Show scroll indicator if needed
	if end < len(m.options) {
		b.WriteString("  ...\n")
	}

	b.WriteString("\n↑/k: Up  ↓/j: Down  Enter: Select  q/Esc: Cancel")

	return b.String()
}

// selectSnippetWithBubbleTea shows an interactive snippet selector using Bubble Tea
func selectSnippetWithBubbleTea(snippets map[string]*models.Snippet) (string, error) {
	if len(snippets) == 0 {
		return "", fmt.Errorf("no templates found")
	}

	model := newSelectorModel(snippets)

	// Run without alternate screen to keep output inline
	p := tea.NewProgram(model)
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
