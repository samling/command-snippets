package template

import (
	"fmt"
	"strings"

	"github.com/samling/command-snippets/internal/models"

	tea "github.com/charmbracelet/bubbletea"
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

		// Check for paste event - Bubble Tea sends pastes as regular key messages
		// with multiple characters
		keyStr := msg.String()
		if !isEnum && len(keyStr) > 1 && !strings.HasPrefix(keyStr, "ctrl+") &&
			!strings.HasPrefix(keyStr, "alt+") && !strings.HasPrefix(keyStr, "shift+") &&
			keyStr != "tab" && keyStr != "enter" && keyStr != "backspace" &&
			keyStr != "up" && keyStr != "down" && keyStr != "left" && keyStr != "right" &&
			keyStr != "esc" {
			// This is likely pasted content
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

	// Render each field
	for i, field := range m.fields {
		// Field label
		label := field.variable.Name
		if field.variable.Description != "" {
			label = fmt.Sprintf("%s (%s)", field.variable.Name, field.variable.Description)
		}

		// Focus indicator
		focusIndicator := "  "
		if i == m.focusIndex {
			focusIndicator = "> "
		}

		// Check if this is an enum field
		isEnum := len(field.enumOptions) > 0

		// Field value with appropriate display
		var displayValue string
		if isEnum {
			// For enum fields, show all options horizontally with selection brackets
			// We pad non-selected options with spaces to match the width with brackets
			var options []string
			for idx, opt := range field.enumOptions {
				if idx == field.enumIndex {
					// Current selection shown with angle brackets
					options = append(options, "<"+opt+">")
				} else {
					// Pad with spaces to match the width of brackets
					options = append(options, " "+opt+" ")
				}
			}
			displayValue = strings.Join(options, " ")
		} else {
			// For text fields, show the value (no cursor)
			displayValue = field.value
		}

		// Build the line
		b.WriteString(fmt.Sprintf("%s%s: %s", focusIndicator, label, displayValue))

		// Add error message if present
		if field.errorMessage != "" {
			b.WriteString(fmt.Sprintf(" [Error: %s]", field.errorMessage))
		}

		b.WriteString("\n")
	}

	// Add instructions at the bottom
	b.WriteString("\n")
	b.WriteString("Tab/↑↓: Navigate fields  ←→: Select options  Enter: Submit  Esc: Cancel")

	return b.String()
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
	// Create the form model
	model := newFormModel(snippet, presetValues, config)

	// Run the Bubble Tea program without alternate screen to avoid terminal issues
	// This keeps the form inline with the command output
	p := tea.NewProgram(model)
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
