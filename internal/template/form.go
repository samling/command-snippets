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
	cursorPos    int // Current cursor position in the value string
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

		// Initialize field with safe defaults
		defaultValue := variable.DefaultValue
		if defaultValue == "" {
			defaultValue = "" // Explicitly set empty string
		}

		field := formField{
			variable:  variable,
			value:     defaultValue,
			cursorPos: len(defaultValue), // Start cursor at end of default value
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

		// Ensure cursor position is valid
		if field.cursorPos > len(field.value) {
			field.cursorPos = len(field.value)
		}

		// Use preset value if available
		if presetValues != nil {
			if presetValue, exists := presetValues[variable.Name]; exists {
				field.value = presetValue
				field.cursorPos = len(presetValue) // Update cursor position
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
	// Safety check: this shouldn't happen anymore since we skip the form for no variables
	// but keep it for defensive programming
	if len(m.fields) == 0 {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				m.done = true
				return m, tea.Quit
			case "ctrl+c", "esc":
				m.cancelled = true
				return m, tea.Quit
			}
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
		}
		return m, nil
	}

	// Safety check: ensure focusIndex is valid
	if m.focusIndex < 0 {
		m.focusIndex = 0
	} else if m.focusIndex >= len(m.fields) {
		m.focusIndex = len(m.fields) - 1
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		currentField := &m.fields[m.focusIndex]
		isEnum := len(currentField.enumOptions) > 0

		// Safety check: ensure cursor position is valid for current field
		if !isEnum {
			if currentField.cursorPos < 0 {
				currentField.cursorPos = 0
			} else if currentField.cursorPos > len(currentField.value) {
				currentField.cursorPos = len(currentField.value)
			}
		}

		// Handle bracketed paste - it comes through as "[" + content + "]"
		keyStr := msg.String()

		// Check if this is bracketed paste content
		if !isEnum && strings.HasPrefix(keyStr, "[") && strings.HasSuffix(keyStr, "]") && len(keyStr) > 2 {
			// This is bracketed paste - extract the content between brackets
			pastedContent := keyStr[1 : len(keyStr)-1]
			// Insert at cursor position
			currentField.value = currentField.value[:currentField.cursorPos] + pastedContent + currentField.value[currentField.cursorPos:]
			currentField.cursorPos += len(pastedContent)
			return m, nil
		}

		// Check for regular multi-character paste (without brackets)
		if !isEnum && len(keyStr) > 1 && !strings.HasPrefix(keyStr, "ctrl+") &&
			!strings.HasPrefix(keyStr, "alt+") && !strings.HasPrefix(keyStr, "shift+") &&
			keyStr != "tab" && keyStr != "enter" && keyStr != "backspace" &&
			keyStr != "up" && keyStr != "down" && keyStr != "left" && keyStr != "right" &&
			keyStr != "esc" && keyStr != "home" && keyStr != "end" {
			// This is likely pasted content without brackets
			// Insert at cursor position
			currentField.value = currentField.value[:currentField.cursorPos] + keyStr + currentField.value[currentField.cursorPos:]
			currentField.cursorPos += len(keyStr)
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "tab", "down":
			// Always move to next field
			if m.focusIndex < len(m.fields)-1 {
				m.focusIndex++
				// Set cursor to end of new field's value
				newField := &m.fields[m.focusIndex]
				if len(newField.enumOptions) == 0 {
					newField.cursorPos = len(newField.value)
					// Safety check
					if newField.cursorPos < 0 {
						newField.cursorPos = 0
					}
				}
			}

		case "shift+tab", "up":
			// Always move to previous field
			if m.focusIndex > 0 {
				m.focusIndex--
				// Set cursor to end of new field's value
				newField := &m.fields[m.focusIndex]
				if len(newField.enumOptions) == 0 {
					newField.cursorPos = len(newField.value)
					// Safety check
					if newField.cursorPos < 0 {
						newField.cursorPos = 0
					}
				}
			}

		case "left":
			if isEnum {
				// For enum fields, cycle to previous option
				if currentField.enumIndex > 0 {
					currentField.enumIndex--
					currentField.value = currentField.enumOptions[currentField.enumIndex]
				}
			} else {
				// For text fields, move cursor left
				if currentField.cursorPos > 0 {
					currentField.cursorPos--
				}
			}

		case "right":
			if isEnum {
				// For enum fields, cycle to next option
				if currentField.enumIndex < len(currentField.enumOptions)-1 {
					currentField.enumIndex++
					currentField.value = currentField.enumOptions[currentField.enumIndex]
				}
			} else {
				// For text fields, move cursor right
				if currentField.cursorPos < len(currentField.value) {
					currentField.cursorPos++
				}
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
			if !isEnum && currentField.cursorPos > 0 {
				// Delete character before cursor
				currentField.value = currentField.value[:currentField.cursorPos-1] + currentField.value[currentField.cursorPos:]
				currentField.cursorPos--
			}

		case "delete":
			// Delete character at cursor position
			if !isEnum && currentField.cursorPos < len(currentField.value) {
				currentField.value = currentField.value[:currentField.cursorPos] + currentField.value[currentField.cursorPos+1:]
			}

		case "home", "ctrl+a":
			// Move cursor to beginning of field
			if !isEnum {
				currentField.cursorPos = 0
			}

		case "end", "ctrl+e":
			// Move cursor to end of field
			if !isEnum {
				currentField.cursorPos = len(currentField.value)
			}

		case "ctrl+u":
			// Clear the current field (Unix-style line clear)
			if !isEnum {
				currentField.value = ""
				currentField.cursorPos = 0
			}

		case "ctrl+k":
			// Delete from cursor to end of line
			if !isEnum && currentField.cursorPos < len(currentField.value) {
				currentField.value = currentField.value[:currentField.cursorPos]
			}

		case "ctrl+w":
			// Delete word before cursor
			if !isEnum && currentField.cursorPos > 0 {
				// Find start of word
				wordStart := currentField.cursorPos - 1
				for wordStart > 0 && currentField.value[wordStart] == ' ' {
					wordStart--
				}
				for wordStart > 0 && currentField.value[wordStart-1] != ' ' {
					wordStart--
				}
				currentField.value = currentField.value[:wordStart] + currentField.value[currentField.cursorPos:]
				currentField.cursorPos = wordStart
			}

		default:
			// Allow single character typing for non-enum fields
			if !isEnum && len(msg.String()) == 1 {
				// Insert character at cursor position
				currentField.value = currentField.value[:currentField.cursorPos] + msg.String() + currentField.value[currentField.cursorPos:]
				currentField.cursorPos++
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

	// Safety check: this shouldn't happen anymore since we skip the form for no variables
	// but keep it for defensive programming
	if len(m.fields) == 0 {
		b.WriteString("No variables to configure.\n")
		b.WriteString("\n")
		helpText := helpStyle.Render("Enter: Execute  Esc: Cancel")
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

	// Render each field
	for i := range m.fields {
		// Use index to get field to ensure we can modify it if needed
		field := &m.fields[i]

		// Safety check: ensure cursor position is valid
		if len(field.enumOptions) == 0 && field.cursorPos > len(field.value) {
			field.cursorPos = len(field.value)
		}
		if field.cursorPos < 0 {
			field.cursorPos = 0
		}
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
			// For text fields, show the value with cursor indicator when focused
			if i == m.focusIndex {
				// Use block cursor that highlights the character
				cursorStyle := lipgloss.NewStyle().Reverse(true) // Reverse video for block cursor

				if len(field.value) == 0 {
					// Empty field - show block cursor as a space
					displayValue = cursorStyle.Render(" ")
				} else if field.cursorPos >= len(field.value) {
					// Cursor at end - add block cursor after text
					displayValue = field.value + cursorStyle.Render(" ")
				} else if field.cursorPos < 0 {
					// Safety check: invalid cursor position
					field.cursorPos = 0
					if len(field.value) > 0 {
						displayValue = cursorStyle.Render(string(field.value[0])) + field.value[1:]
					} else {
						displayValue = cursorStyle.Render(" ")
					}
				} else {
					// Cursor in middle or at beginning - highlight the character at cursor position
					if field.cursorPos == 0 {
						// Cursor at beginning
						displayValue = cursorStyle.Render(string(field.value[0]))
						if len(field.value) > 1 {
							displayValue += field.value[1:]
						}
					} else {
						// Cursor in middle
						displayValue = field.value[:field.cursorPos] +
							cursorStyle.Render(string(field.value[field.cursorPos])) +
							field.value[field.cursorPos+1:]
					}
				}
			} else {
				// Not focused, just show value
				displayValue = field.value
			}
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
	// Show different help text based on current field type
	var helpText string
	if len(m.fields) > 0 && m.focusIndex >= 0 && m.focusIndex < len(m.fields) {
		currentField := m.fields[m.focusIndex]
		if len(currentField.enumOptions) > 0 {
			helpText = helpStyle.Render("Tab/↑↓: Navigate fields  ←→: Select options  Enter: Submit  Esc: Cancel")
		} else {
			helpText = helpStyle.Render("Tab/↑↓: Navigate  ←→: Move cursor  Home/End: Jump  Ctrl+U: Clear  Enter: Submit  Esc: Cancel")
		}
	} else {
		// No fields - just show basic help
		helpText = helpStyle.Render("Enter: Submit  Esc: Cancel")
	}
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
	// Check if there are any non-computed variables that need user input
	hasUserVariables := false
	for _, variable := range snippet.Variables {
		if !variable.Computed {
			hasUserVariables = true
			break
		}
	}

	// If no user variables, return empty map immediately (no form needed)
	if !hasUserVariables {
		return make(map[string]string), nil
	}

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
