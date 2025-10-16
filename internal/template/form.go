package template

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/regex"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// NoColor is a global flag to disable colors in the TUI
var NoColor bool

// wrapLines takes a slice of lines and wraps any that exceed the given width
func wrapLines(lines []string, maxWidth int) []string {
	var wrapped []string
	for _, line := range lines {
		// Manually wrap lines that exceed the width
		if len(line) > maxWidth {
			// Wrap this line
			for len(line) > 0 {
				if len(line) <= maxWidth {
					wrapped = append(wrapped, line)
					break
				}
				// Find a good break point (prefer spaces)
				breakPoint := maxWidth
				if breakPoint > len(line) {
					breakPoint = len(line)
				}
				// Try to break at a space
				for i := breakPoint - 1; i > breakPoint-20 && i > 0; i-- {
					if line[i] == ' ' {
						breakPoint = i
						break
					}
				}
				wrapped = append(wrapped, line[:breakPoint])
				line = strings.TrimLeft(line[breakPoint:], " ")
			}
		} else {
			wrapped = append(wrapped, line)
		}
	}
	return wrapped
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

	regexExplanationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)

	regexTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	commandPreviewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")). // Cyan
				Padding(0, 0).
				MarginBottom(1)

	commandPreviewTitleStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("86")).
					Bold(true)

	unfilledVarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")). // Orange for unfilled variables
				Bold(true)

	filledVarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("120")) // Green for filled variables
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
	snippet           *models.Snippet
	fields            []formField
	focusIndex        int
	done              bool
	cancelled         bool
	config            *models.Config
	width             int
	height            int
	showRegexPane     bool // Whether to show regex explanation pane
	regexPaneScrollUp int  // Number of lines scrolled up in regex pane
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
		snippet:       snippet,
		fields:        fields,
		focusIndex:    0,
		config:        config,
		showRegexPane: true, // Show regex pane by default
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

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
			// Reset scroll when pasting
			m.regexPaneScrollUp = 0
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
			// Reset scroll when pasting
			m.regexPaneScrollUp = 0
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "ctrl+r":
			// Toggle regex pane visibility
			m.showRegexPane = !m.showRegexPane
			m.regexPaneScrollUp = 0 // Reset scroll when toggling

		case "ctrl+u":
			// Scroll regex pane up (show earlier content)
			if currentField.variable.Type == "regex" && currentField.value != "" && m.showRegexPane {
				m.regexPaneScrollUp -= 5
				if m.regexPaneScrollUp < 0 {
					m.regexPaneScrollUp = 0
				}
				return m, nil // Consume the event to prevent default scrolling
			}

		case "ctrl+d":
			// Scroll regex pane down (show later content)
			if currentField.variable.Type == "regex" && currentField.value != "" && m.showRegexPane && m.height > 0 && m.width >= 100 {
				// Calculate max scroll to prevent scrolling past content
				// Must use same calculation as View()
				formWidth := int(float64(m.width) * 0.6)
				if formWidth < 60 {
					formWidth = 60
				}
				explanationWidth := m.width - formWidth - 2

				explanation := regex.ExplainRegexPattern(currentField.value)
				rawLines := strings.Split(strings.TrimRight(explanation, "\n"), "\n")
				explanationLines := wrapLines(rawLines, explanationWidth-4)

				maxContentLines := m.height - 5 // Must match View() calculation
				if maxContentLines < 5 {
					maxContentLines = 5
				}
				maxScroll := len(explanationLines) - maxContentLines
				if maxScroll < 0 {
					maxScroll = 0
				}

				// Only increment if we're not already at max
				if m.regexPaneScrollUp < maxScroll {
					m.regexPaneScrollUp += 5
					if m.regexPaneScrollUp > maxScroll {
						m.regexPaneScrollUp = maxScroll
					}
				}
				return m, nil // Consume the event to prevent default scrolling
			}

		case "tab", "down":
			// Move to next field, wrap around to top
			m.focusIndex++
			if m.focusIndex >= len(m.fields) {
				m.focusIndex = 0
			}
			// Set cursor to end of new field's value
			newField := &m.fields[m.focusIndex]
			if len(newField.enumOptions) == 0 {
				newField.cursorPos = len(newField.value)
				// Safety check
				if newField.cursorPos < 0 {
					newField.cursorPos = 0
				}
			}
			// Reset scroll when changing fields
			m.regexPaneScrollUp = 0

		case "shift+tab", "up":
			// Move to previous field, wrap around to bottom
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = len(m.fields) - 1
			}
			// Set cursor to end of new field's value
			newField := &m.fields[m.focusIndex]
			if len(newField.enumOptions) == 0 {
				newField.cursorPos = len(newField.value)
				// Safety check
				if newField.cursorPos < 0 {
					newField.cursorPos = 0
				}
			}
			// Reset scroll when changing fields
			m.regexPaneScrollUp = 0

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
				// Reset scroll when modifying content
				m.regexPaneScrollUp = 0
			}

		case "delete":
			// Delete character at cursor position
			if !isEnum && currentField.cursorPos < len(currentField.value) {
				currentField.value = currentField.value[:currentField.cursorPos] + currentField.value[currentField.cursorPos+1:]
				// Reset scroll when modifying content
				m.regexPaneScrollUp = 0
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

		case "ctrl+x":
			// Clear the current field
			if !isEnum {
				currentField.value = ""
				currentField.cursorPos = 0
				// Reset scroll when modifying content
				m.regexPaneScrollUp = 0
			}

		case "ctrl+y":
			// Delete from cursor to end of line (rebind from ctrl+k)
			if !isEnum && currentField.cursorPos < len(currentField.value) {
				currentField.value = currentField.value[:currentField.cursorPos]
				// Reset scroll when modifying content
				m.regexPaneScrollUp = 0
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
				// Reset scroll when modifying content
				m.regexPaneScrollUp = 0
			}

		default:
			// Allow single character typing for non-enum fields
			if !isEnum && len(msg.String()) == 1 {
				// Insert character at cursor position
				currentField.value = currentField.value[:currentField.cursorPos] + msg.String() + currentField.value[currentField.cursorPos:]
				currentField.cursorPos++
				// Reset scroll when typing
				m.regexPaneScrollUp = 0
			}
		}
	}

	return m, nil
}

// applyTransformation applies variable transformations for preview purposes
func (m formModel) applyTransformation(variable models.Variable, value string, allValues map[string]string) string {
	// Determine which transform to use
	var transform *models.Transform

	// Use transformTemplate if specified
	if variable.TransformTemplate != "" && m.config != nil {
		if tmplDef, exists := m.config.TransformTemplates[variable.TransformTemplate]; exists {
			transform = tmplDef.Transform
		}
	} else if variable.Transform != nil {
		transform = variable.Transform
	}

	// Handle computed variables with compose first
	if variable.Computed && transform != nil && transform.Compose != "" {
		tmpl, err := template.New("compose").Parse(transform.Compose)
		if err != nil {
			// On error, return empty string for preview
			return ""
		}

		var buf strings.Builder
		if err := tmpl.Execute(&buf, allValues); err != nil {
			// On error, return empty string for preview
			return ""
		}
		return buf.String()
	}

	// Handle transformations
	if transform != nil {
		// Boolean transformations
		if variable.Type == "boolean" {
			if value == "true" || value == "yes" || value == "1" {
				return transform.TrueValue
			}
			return transform.FalseValue
		}

		// Regular transformations
		if value == "" && transform.EmptyValue != "" {
			return transform.EmptyValue
		} else if value != "" && transform.ValuePattern != "" {
			// Use Go template for value pattern
			tmpl, err := template.New("transform").Parse(transform.ValuePattern)
			if err != nil {
				// Fallback to simple replacement on error
				return strings.ReplaceAll(transform.ValuePattern, "{{.Value}}", value)
			}

			var buf strings.Builder
			data := map[string]string{"Value": value}
			if err := tmpl.Execute(&buf, data); err != nil {
				// Fallback to simple replacement on error
				return strings.ReplaceAll(transform.ValuePattern, "{{.Value}}", value)
			}
			return buf.String()
		}
	}

	// Use default value if empty
	if value == "" {
		return variable.DefaultValue
	}

	return value
}

// renderCommandPreview generates a preview of the command with current values
func (m formModel) renderCommandPreview() string {
	if m.snippet == nil {
		return ""
	}

	command := m.snippet.Command
	result := command

	// Build a map of variable values for quick lookup (only non-computed variables)
	valueMap := make(map[string]string)
	filledMap := make(map[string]bool)
	for _, field := range m.fields {
		valueMap[field.variable.Name] = field.value
		filledMap[field.variable.Name] = field.value != ""
	}

	// Replace each variable placeholder with styled version
	for _, variable := range m.snippet.Variables {
		placeholder := fmt.Sprintf("<%s>", variable.Name)

		if !strings.Contains(result, placeholder) {
			continue
		}

		// For computed variables, we don't have a raw value from fields
		rawValue := ""
		isFilled := false
		if !variable.Computed {
			rawValue = valueMap[variable.Name]
			isFilled = filledMap[variable.Name]
		}

		// Apply transformations to get the actual value that would be used
		transformedValue := m.applyTransformation(variable, rawValue, valueMap)

		// Create the styled replacement
		var replacement string
		if variable.Computed {
			// For computed variables, show the result or placeholder
			if transformedValue != "" {
				// Successfully computed - show in green
				replacement = filledVarStyle.Render(transformedValue)
			} else {
				// Computation failed or dependencies not ready - show placeholder in orange
				replacement = unfilledVarStyle.Render(placeholder)
			}
		} else if transformedValue != "" {
			// Show transformed value in green if non-empty
			replacement = filledVarStyle.Render(transformedValue)
		} else if isFilled && rawValue != "" {
			// Field is filled but transformation produced empty string - show nothing (empty)
			replacement = ""
		} else {
			// Show placeholder if empty and not filled
			replacement = unfilledVarStyle.Render(placeholder)
		}

		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	// Build the preview box
	var b strings.Builder
	b.WriteString(commandPreviewTitleStyle.Render("Command Preview:"))
	b.WriteString("\n")
	b.WriteString(result)

	return commandPreviewStyle.Render(b.String())
}

// View renders the form
func (m formModel) View() string {
	if m.done || m.cancelled {
		return ""
	}

	// Safety check: this shouldn't happen anymore since we skip the form for no variables
	// but keep it for defensive programming
	if len(m.fields) == 0 {
		var b strings.Builder
		b.WriteString("No variables to configure.\n")
		b.WriteString("\n")
		helpText := helpStyle.Render("Enter: Execute  Esc: Cancel")
		if m.width > 0 {
			helpText = lipgloss.NewStyle().Width(m.width).Render(helpText)
		}
		b.WriteString(helpText)
		return b.String()
	}

	// Check if current field is a regex field with content and pane is enabled
	var regexExplanation string
	var showPane bool
	if m.focusIndex >= 0 && m.focusIndex < len(m.fields) {
		currentField := m.fields[m.focusIndex]
		if currentField.variable.Type == "regex" && currentField.value != "" && m.showRegexPane {
			regexExplanation = regex.ExplainRegexPattern(currentField.value)
			// Only show pane if terminal is wide enough (at least 100 chars)
			showPane = m.width >= 100
		}
	}

	// Determine layout widths
	// Start with full width, only split if we're actually showing the pane
	formWidth := m.width
	if showPane && regexExplanation != "" {
		// Split the width: 60% for form, 40% for explanation
		formWidth = int(float64(m.width) * 0.6)
	}
	// If formWidth is 0 or negative (shouldn't happen but safety check), use full width
	if formWidth <= 0 {
		formWidth = m.width
	}

	// Build the form fields
	var formBuilder strings.Builder

	// Add command preview at the top
	commandPreview := m.renderCommandPreview()
	if commandPreview != "" {
		if formWidth > 0 {
			commandPreview = lipgloss.NewStyle().Width(formWidth).Render(commandPreview)
		}
		formBuilder.WriteString(commandPreview)
		formBuilder.WriteString("\n")
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

		// Build the line with wrapping
		line := fmt.Sprintf("%s%s %s", linePrefix, styledLabel, displayValue)

		// Apply width constraint for proper wrapping (formWidth is either split width or full width)
		if formWidth > 0 {
			wrappedLine := lipgloss.NewStyle().Width(formWidth).Render(line)
			formBuilder.WriteString(wrappedLine)
		} else {
			formBuilder.WriteString(line)
		}
		formBuilder.WriteString("\n")

		// Add error message if present
		if field.errorMessage != "" {
			errorLine := "    " + errorStyle.Render("[Error: "+field.errorMessage+"]")
			if formWidth > 0 {
				errorLine = lipgloss.NewStyle().Width(formWidth).Render(errorLine)
			}
			formBuilder.WriteString(errorLine)
			formBuilder.WriteString("\n")
		}
	}

	// Add instructions at the bottom of the form
	formBuilder.WriteString("\n")
	// Show different help text based on current field type
	var helpText string
	if len(m.fields) > 0 && m.focusIndex >= 0 && m.focusIndex < len(m.fields) {
		currentField := m.fields[m.focusIndex]
		if len(currentField.enumOptions) > 0 {
			helpText = helpStyle.Render("Tab/↑↓: Navigate  ←→: Select  Enter: Submit  Esc: Cancel")
		} else if currentField.variable.Type == "regex" {
			// Show regex-specific help
			paneStatus := "on"
			if !m.showRegexPane {
				paneStatus = "off"
			}
			helpText = helpStyle.Render(fmt.Sprintf("Tab/↑↓: Navigate  Ctrl+X: Clear  Ctrl+R: Pane(%s)  Ctrl+U/D: Scroll  Enter: Submit  Esc: Cancel", paneStatus))
		} else {
			helpText = helpStyle.Render("Tab/↑↓: Navigate  ←→: Move cursor  Home/End: Jump  Ctrl+X: Clear  Enter: Submit  Esc: Cancel")
		}
	} else {
		// No fields - just show basic help
		helpText = helpStyle.Render("Enter: Submit  Esc: Cancel")
	}
	if formWidth > 0 {
		helpText = lipgloss.NewStyle().Width(formWidth).Render(helpText)
	}
	formBuilder.WriteString(helpText)

	formContent := formBuilder.String()

	// If we have a regex explanation and should show the pane, render it in a side pane
	if showPane && regexExplanation != "" {
		explanationWidth := m.width - formWidth - 2 // 2 for padding/border

		// Split explanation into lines and wrap them to fit the pane width
		rawLines := strings.Split(strings.TrimRight(regexExplanation, "\n"), "\n")
		explanationLines := wrapLines(rawLines, explanationWidth-4)

		// Calculate the maximum height available for the pane content
		// The pane should be the FULL terminal height since it's side-by-side with the form
		// Pane structure: title (1) + top indicator (1) + content (N) + bottom indicator (1) + borders (2)
		// Total pane lines = N + 5, so N = m.height - 5
		maxContentLines := m.height - 5 // Full height minus title, indicators, and borders
		if maxContentLines < 5 {
			maxContentLines = 5 // Minimum readable height
		}

		// Limit scroll based on actual content
		// If we have 20 lines and can show 15, max scroll is 5 (to show lines 5-20)
		maxScroll := len(explanationLines) - maxContentLines
		if maxScroll < 0 {
			maxScroll = 0
		}

		// Strictly clamp scroll position - don't allow scrolling past the end
		if m.regexPaneScrollUp > maxScroll {
			m.regexPaneScrollUp = maxScroll
		}
		if m.regexPaneScrollUp < 0 {
			m.regexPaneScrollUp = 0
		}

		// Calculate visible window - STRICTLY limit to maxContentLines
		startLine := m.regexPaneScrollUp

		// Build explanation as a fixed-line-count structure
		scrollIndicator := ""
		if len(explanationLines) > maxContentLines {
			scrollIndicator = fmt.Sprintf(" (%d/%d)", startLine+1, len(explanationLines))
		}

		// Check if there's more content above or below
		hasContentAbove := startLine > 0
		// Content below exists if we can't show all remaining lines
		hasContentBelow := (startLine + maxContentLines) < len(explanationLines)

		// Build exactly the right number of lines - structure must be EXACTLY the same every time
		var paneLines []string

		// Line 1: Title
		paneLines = append(paneLines, regexTitleStyle.Render("Pattern Explanation"+scrollIndicator))

		// Line 2: Top indicator or blank (MUST be exactly 1 line, no styling)
		if hasContentAbove {
			paneLines = append(paneLines, "        ↑ more above ↑")
		} else {
			paneLines = append(paneLines, " ")
		}

		// Lines 3 to 3+maxContentLines: Content (MUST be exactly maxContentLines)
		for i := 0; i < maxContentLines; i++ {
			lineIdx := startLine + i
			if lineIdx < len(explanationLines) {
				paneLines = append(paneLines, explanationLines[lineIdx])
			} else {
				paneLines = append(paneLines, " ")
			}
		}

		// Last line: Bottom indicator or blank (MUST be exactly 1 line, no styling)
		if hasContentBelow {
			paneLines = append(paneLines, "        ↓ more below ↓")
		} else {
			paneLines = append(paneLines, " ")
		}

		// Verify we have exactly maxContentLines + 3 lines
		expectedLines := maxContentLines + 3
		if len(paneLines) != expectedLines {
			// Safety: force exact line count
			for len(paneLines) < expectedLines {
				paneLines = append(paneLines, " ")
			}
			if len(paneLines) > expectedLines {
				paneLines = paneLines[:expectedLines]
			}
		}

		// Join and render WITHOUT height constraint - let the line count control it
		paneContent := strings.Join(paneLines, "\n")
		explanationContent := regexExplanationStyle.
			Width(explanationWidth).
			UnsetHeight().
			UnsetMaxHeight().
			Render(paneContent)

		// Join form and explanation horizontally
		return lipgloss.JoinHorizontal(lipgloss.Top, formContent, explanationContent)
	}

	return formContent
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
