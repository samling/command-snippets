package template

import (
	"fmt"
	"os"
	"strings"

	"github.com/samling/command-snippets/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// NoColor is a global flag to disable colors in the TUI
var NoColor bool

// wrapLine wraps a line to fit within the given width, indenting continuation lines
func wrapLine(line string, width int, indent string) []string {
	if width <= 0 || len(line) <= width {
		return []string{line}
	}

	var result []string
	remaining := line
	firstLine := true

	for len(remaining) > 0 {
		availWidth := width
		if !firstLine {
			availWidth = width - len(indent)
		}

		if len(remaining) <= availWidth {
			if firstLine {
				result = append(result, remaining)
			} else {
				result = append(result, indent+remaining)
			}
			break
		}

		// Find a good break point (space, comma, etc.)
		breakPoint := availWidth
		for i := availWidth - 1; i > availWidth/2; i-- {
			if remaining[i] == ' ' || remaining[i] == ',' || remaining[i] == '-' {
				breakPoint = i + 1
				break
			}
		}

		if firstLine {
			result = append(result, remaining[:breakPoint])
			firstLine = false
		} else {
			result = append(result, indent+remaining[:breakPoint])
		}
		remaining = strings.TrimSpace(remaining[breakPoint:])
	}

	return result
}

// Style definitions
var (
	focusedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")) // Pink/magenta for focused items

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")) // Gray for labels

	selectedEnumStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")). // Cyan for selected enum
				Bold(true)

	unselectedEnumStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("247")) // Light gray for unselected enums

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red for errors

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")) // Gray for help text
)

// formField represents a single field in the form
type formField struct {
	variable     models.Variable
	value        string
	errorMessage string
	enumIndex    int      // For enum fields, tracks the selected option index
	enumOptions  []string // For enum/boolean fields, the available options
}

// formModel represents the state of the form
type formModel struct {
	fields     []formField
	focusIndex int
	done       bool
	cancelled  bool
	config     *models.Config
	width      int
	height     int
}

// newFormModel creates a new form model for the given snippet
func newFormModel(snippet *models.Snippet, presetValues map[string]string, config *models.Config) formModel {
	var fields []formField

	for _, variable := range snippet.Variables {
		if variable.Computed {
			continue // Skip computed variables
		}

		field := formField{
			variable:  variable,
			value:     variable.DefaultValue,
			enumIndex: 0,
		}

		// Set up enum options for boolean or enum fields
		if variable.Type == "boolean" {
			field.enumOptions = []string{"false", "true"}
			// Set default value for boolean if not specified
			if field.value == "" {
				field.value = "false"
			}
		} else if variable.Validation != nil && len(variable.Validation.Enum) > 0 {
			field.enumOptions = variable.Validation.Enum
		}

		// Use preset value if available
		if presetValues != nil {
			if presetValue, exists := presetValues[variable.Name]; exists {
				field.value = presetValue
			}
		}

		// For fields with enum options, set the initial index based on value
		if len(field.enumOptions) > 0 {
			for i, option := range field.enumOptions {
				if option == field.value {
					field.enumIndex = i
					break
				}
			}
			// Ensure value is set to a valid option
			if field.enumIndex < len(field.enumOptions) {
				field.value = field.enumOptions[field.enumIndex]
			}
		}

		fields = append(fields, field)
	}

	return formModel{
		fields:     fields,
		focusIndex: 0,
		config:     config,
	}
}

// Init initializes the model
func (m formModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		currentField := &m.fields[m.focusIndex]
		isEnum := len(currentField.enumOptions) > 0

		// Handle bracketed paste - it comes through as "[" + content + "]"
		keyStr := msg.String()

		// Check if this is bracketed paste content
		if !isEnum && strings.HasPrefix(keyStr, "[") && strings.HasSuffix(keyStr, "]") && len(keyStr) > 2 {
			// This is bracketed paste - extract the content between brackets
			pastedContent := keyStr[1 : len(keyStr)-1]
			currentField.value += pastedContent
			return m, nil
		}

		// Check for regular multi-character paste (without brackets)
		if !isEnum && len(keyStr) > 1 && !strings.HasPrefix(keyStr, "ctrl+") &&
			!strings.HasPrefix(keyStr, "alt+") && !strings.HasPrefix(keyStr, "shift+") &&
			keyStr != "tab" && keyStr != "enter" && keyStr != "backspace" &&
			keyStr != "up" && keyStr != "down" && keyStr != "left" && keyStr != "right" &&
			keyStr != "esc" {
			// This is likely pasted content without brackets
			currentField.value += keyStr
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "tab", "down", "j":
			// Always move to next field
			if m.focusIndex < len(m.fields)-1 {
				m.focusIndex++
			}

		case "shift+tab", "up", "k":
			// Always move to previous field
			if m.focusIndex > 0 {
				m.focusIndex--
			}

		case "left":
			// For enum fields, cycle to previous option
			if isEnum && currentField.enumIndex > 0 {
				currentField.enumIndex--
				currentField.value = currentField.enumOptions[currentField.enumIndex]
			}

		case "right":
			// For enum fields, cycle to next option
			if isEnum && currentField.enumIndex < len(currentField.enumOptions)-1 {
				currentField.enumIndex++
				currentField.value = currentField.enumOptions[currentField.enumIndex]
			}

		case "h":
			// Only use h for navigation in enum fields, otherwise treat as text
			if isEnum && currentField.enumIndex > 0 {
				currentField.enumIndex--
				currentField.value = currentField.enumOptions[currentField.enumIndex]
			} else if !isEnum {
				// For text fields, treat as regular character
				currentField.value += "h"
			}

		case "l":
			// Only use l for navigation in enum fields, otherwise treat as text
			if isEnum && currentField.enumIndex < len(currentField.enumOptions)-1 {
				currentField.enumIndex++
				currentField.value = currentField.enumOptions[currentField.enumIndex]
			} else if !isEnum {
				// For text fields, treat as regular character
				currentField.value += "l"
			}

		case "enter":
			// Submit form if on last field, otherwise move to next
			if m.focusIndex == len(m.fields)-1 {
				// Validate all fields before submitting
				allValid := true
				for i := range m.fields {
					if err := m.fields[i].variable.ValidateWithConfig(m.fields[i].value, m.config); err != nil {
						m.fields[i].errorMessage = err.Error()
						allValid = false
					} else {
						m.fields[i].errorMessage = ""
					}
				}

				if allValid {
					m.done = true
					return m, tea.Quit
				}
			} else {
				// Move to next field
				m.focusIndex++
			}

		case "backspace":
			// Only allow backspace for non-enum fields
			if !isEnum && len(currentField.value) > 0 {
				currentField.value = currentField.value[:len(currentField.value)-1]
			}

		case "ctrl+u":
			// Clear the current field (Unix-style line clear)
			if !isEnum {
				currentField.value = ""
			}

		default:
			// Allow single character typing for non-enum fields
			if !isEnum && len(msg.String()) == 1 {
				currentField.value += msg.String()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View renders the form
func (m formModel) View() string {
	if m.done || m.cancelled {
		return ""
	}

	var b strings.Builder

	// Create a style with max width if we have terminal width
	var contentStyle lipgloss.Style
	if m.width > 0 {
		contentStyle = lipgloss.NewStyle().MaxWidth(m.width)
	} else {
		contentStyle = lipgloss.NewStyle()
	}

	// Render each field
	for i, field := range m.fields {
		// Field label
		label := field.variable.Name
		if field.variable.Description != "" {
			label = fmt.Sprintf("%s (%s)", field.variable.Name, field.variable.Description)
		}

		// Focus indicator and label styling
		var linePrefix string
		var styledLabel string
		if i == m.focusIndex {
			linePrefix = focusedStyle.Render("> ")
			styledLabel = focusedStyle.Render(label + ":")
		} else {
			linePrefix = "  "
			styledLabel = labelStyle.Render(label + ":")
		}

		// Check if this is an enum field
		isEnum := len(field.enumOptions) > 0

		// Field value with appropriate display
		var displayValue string
		if isEnum {
			// For enum fields, show all options horizontally with selection brackets
			var options []string
			for idx, opt := range field.enumOptions {
				if idx == field.enumIndex {
					// Current selection shown with angle brackets and color
					options = append(options, selectedEnumStyle.Render("<"+opt+">"))
				} else {
					// Unselected options with padding
					options = append(options, unselectedEnumStyle.Render(" "+opt+" "))
				}
			}
			displayValue = strings.Join(options, " ")
		} else {
			// For text fields, just show the value without styling (will be white when focused)
			displayValue = field.value
		}

		// Build the line
		line := fmt.Sprintf("%s%s %s", linePrefix, styledLabel, displayValue)

		// Use Lip Gloss to handle wrapping if width is set
		if m.width > 0 {
			// Wrap the line at terminal width
			wrappedLine := lipgloss.NewStyle().Width(m.width).Render(line)
			b.WriteString(wrappedLine)
			b.WriteString("\n")
		} else {
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Add error message if present
		if field.errorMessage != "" {
			errorLine := "    " + errorStyle.Render("[Error: "+field.errorMessage+"]")
			if m.width > 0 {
				errorLine = lipgloss.NewStyle().Width(m.width).Render(errorLine)
			}
			b.WriteString(errorLine + "\n")
		}
	}

	// Add instructions at the bottom
	b.WriteString("\n")
	helpText := helpStyle.Render("Tab/↑↓: Navigate fields  ←→: Select options  Enter: Submit  Esc: Cancel")
	if m.width > 0 {
		helpText = lipgloss.NewStyle().Width(m.width).Render(helpText)
	}
	b.WriteString(helpText)

	// Apply the overall width constraint
	output := b.String()
	if m.width > 0 {
		return contentStyle.Render(output)
	}
	return output
}

// getValues returns the form values as a map
func (m formModel) getValues() map[string]string {
	values := make(map[string]string)
	for _, field := range m.fields {
		values[field.variable.Name] = field.value
	}
	return values
}

// promptForVariablesWithBubbleTea shows a Bubble Tea form for all variables
func promptForVariablesWithBubbleTea(snippet *models.Snippet, presetValues map[string]string, config *models.Config) (map[string]string, error) {
	// Force color output when stderr is a TTY (even in subshells), unless --no-color
	if !NoColor && term.IsTerminal(int(os.Stderr.Fd())) {
		// Detect the best color profile for the terminal
		output := termenv.NewOutput(os.Stderr)
		lipgloss.SetColorProfile(output.Profile)
	} else if NoColor {
		// Disable colors
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	// Get terminal width for wrapping
	width, _, _ := term.GetSize(int(os.Stderr.Fd()))
	if width == 0 {
		width = 80 // Default width
	}

	// Create the form model
	model := newFormModel(snippet, presetValues, config)
	model.width = width

	// Run the Bubble Tea program with alternate screen for better UX
	// Use stderr for the TUI so stdout can be captured for the command output
	p := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running form: %w", err)
	}

	// Check if cancelled
	form := finalModel.(formModel)
	if form.cancelled {
		return nil, fmt.Errorf("form cancelled")
	}

	// Return the values
	return form.getValues(), nil
}
