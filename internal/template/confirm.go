package template

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// confirmModel represents a simple yes/no confirmation dialog
type confirmModel struct {
	message   string
	confirmed bool
	cancelled bool
	done      bool
}

// newConfirmModel creates a new confirmation model
func newConfirmModel(message string) confirmModel {
	return confirmModel{
		message: message,
	}
}

// Init initializes the model
func (m confirmModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y", "yes":
			m.confirmed = true
			m.done = true
			return m, tea.Quit
		case "n", "no":
			m.confirmed = false
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the confirmation prompt
func (m confirmModel) View() string {
	if m.done {
		return ""
	}
	return m.message + " [y/n]: "
}

// promptForConfirmation shows a yes/no confirmation dialog
func promptForConfirmation(message string) (bool, error) {
	model := newConfirmModel(message)

	// Use stderr for the TUI so stdout can be captured for command output
	p := tea.NewProgram(model, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	confirm := finalModel.(confirmModel)
	if confirm.cancelled {
		// Exit silently on cancellation
		os.Exit(0)
	}

	return confirm.confirmed, nil
}
