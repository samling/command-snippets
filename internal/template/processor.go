package template

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/huh"
	"github.com/samling/command-snippets/internal/models"

	"github.com/AlecAivazis/survey/v2"
)

// ExecutionMode defines how commands should be executed
type ExecutionMode int

const (
	CopyToClipboard ExecutionMode = iota // Copy command to clipboard (default)
	PrintOnly                            // Print command only
	AutoExecute                          // Execute automatically without prompting
	PromptExecute                        // Prompt before executing (original behavior)
)

// Processor handles snippet template processing
type Processor struct {
	config *models.Config
}

// getTerminalIO returns file handles for direct terminal access
// This ensures interactive prompts work even when stdout is redirected
func getTerminalIO() (*os.File, *os.File, error) {
	// Try to open /dev/tty for direct terminal access
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		// Fallback to stdin/stderr if /dev/tty is not available
		return os.Stdin, os.Stderr, nil
	}
	return tty, tty, nil
}

// isSurveyUserCancellation checks if a survey error represents user cancellation
func isSurveyUserCancellation(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Common survey cancellation error messages
	return errStr == "interrupt" ||
		errStr == "terminal: interrupt" ||
		strings.Contains(errStr, "interrupt") ||
		strings.Contains(errStr, "EOF")
}

// isHuhUserCancellation checks if a huh error represents user cancellation
func isHuhUserCancellation(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Common huh cancellation error messages
	return errStr == "user aborted" ||
		errStr == "interrupt" ||
		strings.Contains(errStr, "user aborted") ||
		strings.Contains(errStr, "interrupt") ||
		strings.Contains(errStr, "EOF")
}

// NewProcessor creates a new template processor
func NewProcessor(config *models.Config) *Processor {
	return &Processor{
		config: config,
	}
}

// ExecuteWithMode prompts for variables and handles execution based on specified mode
func (p *Processor) ExecuteWithMode(snippet *models.Snippet, mode ExecutionMode) error {
	return p.ExecuteWithModeAndPresets(snippet, mode, nil)
}

// ExecuteWithModeAndPresets prompts for variables (skipping preset ones) and handles execution
func (p *Processor) ExecuteWithModeAndPresets(snippet *models.Snippet, mode ExecutionMode, presetValues map[string]string) error {
	values, err := p.promptForVariablesWithPresets(snippet, presetValues)
	if err != nil {
		return err
	}

	command, err := snippet.ProcessTemplate(values, p.config)
	if err != nil {
		return err
	}

	// Handle execution based on mode
	switch mode {
	case CopyToClipboard:
		// Copy command to clipboard (default behavior)
		err := clipboard.WriteAll(command)
		if err != nil {
			// Fallback to printing if clipboard fails
			fmt.Fprintf(os.Stderr, "Failed to copy to clipboard: %v\n", err)
			fmt.Fprintf(os.Stderr, "Command: %s\n", command)
		} else {
			fmt.Fprintf(os.Stderr, "Command copied to clipboard: %s\n", command)
		}
		return nil

	case PrintOnly:
		// Print just the raw command (perfect for piping)
		fmt.Print(command)
		return nil

	case AutoExecute:
		// Show command with prefix, then execute
		fmt.Fprintf(os.Stderr, "Command: %s\n", command)
		return p.executeCommand(command)

	case PromptExecute:
		// Show command with prefix, then ask for confirmation
		fmt.Fprintf(os.Stderr, "Command: %s\n", command)

		// Get direct terminal access for confirmation prompt
		termIn, termOut, err := getTerminalIO()
		if err != nil {
			return fmt.Errorf("cannot access terminal: %w", err)
		}

		confirm := false
		prompt := &survey.Confirm{
			Message: "Execute this command?",
		}
		stdio := survey.WithStdio(termIn, termOut, termOut)
		if err := survey.AskOne(prompt, &confirm, stdio); err != nil {
			// Handle survey interrupts and cancellations as user cancellation
			if isSurveyUserCancellation(err) {
				os.Exit(0)
			}
			return err
		}
		if !confirm {
			return nil
		}
		return p.executeCommand(command)

	default:
		return fmt.Errorf("unknown execution mode: %v", mode)
	}
}

// InteractiveExecute prompts for variables and executes a snippet (legacy method)
// This maintains backward compatibility and uses the PromptExecute mode
func (p *Processor) InteractiveExecute(snippet *models.Snippet) error {
	return p.ExecuteWithMode(snippet, PromptExecute)
}

// ProcessSnippet processes a snippet with given values (non-interactive)
func (p *Processor) ProcessSnippet(snippet *models.Snippet, values map[string]string) (string, error) {
	return snippet.ProcessTemplate(values, p.config)
}

// promptForVariables interactively prompts for snippet variables
func (p *Processor) promptForVariables(snippet *models.Snippet) (map[string]string, error) {
	values := make(map[string]string)

	// Prompt for each variable defined in the snippet
	for _, variable := range snippet.Variables {
		if variable.Computed {
			continue // Skip computed variables
		}

		// Loop until we get valid input
		for {
			value, err := p.promptForVariable(variable)
			if err != nil {
				// Handle survey interrupts and cancellations as user cancellation
				if isSurveyUserCancellation(err) {
					os.Exit(0)
				}
				return nil, err
			}

			// Validate the value (using config for type-based validation)
			if err := variable.ValidateWithConfig(value, p.config); err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v\n", err)
				fmt.Fprintln(os.Stderr, "Please try again.")
				continue // Reprompt for this variable
			}

			// Valid input - store it and move to next variable
			values[variable.Name] = value
			break
		}
	}

	return values, nil
}

// promptForVariablesWithPresets interactively prompts for snippet variables, using preset values where available
func (p *Processor) promptForVariablesWithPresets(snippet *models.Snippet, presetValues map[string]string) (map[string]string, error) {
	values := make(map[string]string)

	// Initialize with preset values
	for key, value := range presetValues {
		values[key] = value
	}

	// Collect variables that need prompting
	var variablesToPrompt []models.Variable
	for _, variable := range snippet.Variables {
		if variable.Computed {
			continue // Skip computed variables
		}

		// Skip if already set via --set
		if _, exists := values[variable.Name]; exists {
			// Validate the preset value
			if err := variable.ValidateWithConfig(values[variable.Name], p.config); err != nil {
				return nil, fmt.Errorf("preset value for '%s': %w", variable.Name, err)
			}
			continue
		}

		variablesToPrompt = append(variablesToPrompt, variable)
	}

	// If no variables need prompting, return early
	if len(variablesToPrompt) == 0 {
		return values, nil
	}

	// Use the new form-based UI
	promptedValues, err := p.promptVariablesForm(variablesToPrompt)
	if err != nil {
		return nil, err
	}

	// Merge prompted values with existing values
	for key, value := range promptedValues {
		values[key] = value
	}

	return values, nil
}

// promptVariablesForm creates a form UI showing all variables at once using huh
func (p *Processor) promptVariablesForm(variables []models.Variable) (map[string]string, error) {
	if len(variables) == 0 {
		return make(map[string]string), nil
	}

	// Create value storage - huh needs pointers to bind values
	valuePointers := make(map[string]*string)
	boolValues := make(map[string]*bool)

	for _, variable := range variables {
		if variable.Type == "boolean" {
			defaultBool := variable.DefaultValue == "true"
			boolValues[variable.Name] = &defaultBool
		} else {
			defaultValue := variable.DefaultValue
			valuePointers[variable.Name] = &defaultValue
		}
	}

	// Build form fields
	var fields []huh.Field

	for _, variable := range variables {
		// Build title
		title := variable.Name
		if variable.Description != "" {
			title = fmt.Sprintf("%s (%s)", variable.Name, variable.Description)
		}

		// Handle different variable types
		switch variable.Type {
		case "boolean":
			field := huh.NewConfirm().
				Key(variable.Name).
				Title(title).
				Value(boolValues[variable.Name])
			fields = append(fields, field)

		default:
			// Handle enum types with select
			if variable.Validation != nil && len(variable.Validation.Enum) > 0 {
				// Convert strings to huh options
				var options []huh.Option[string]
				for _, option := range variable.Validation.Enum {
					options = append(options, huh.NewOption(option, option))
				}

				field := huh.NewSelect[string]().
					Key(variable.Name).
					Title(title).
					Options(options...).
					Value(valuePointers[variable.Name])
				fields = append(fields, field)
			} else {
				// Regular string input with validation
				field := huh.NewInput().
					Key(variable.Name).
					Title(title).
					Value(valuePointers[variable.Name]).
					Validate(func(str string) error {
						return variable.ValidateWithConfig(str, p.config)
					})
				fields = append(fields, field)
			}
		}
	}

	// Create and run the form with height limit for scrolling in small terminals
	form := huh.NewForm(
		huh.NewGroup(fields...),
	)

	// Set height for scrolling in small terminals (default 8, customizable via env var)
	formHeight := 8
	if heightEnv := os.Getenv("CS_FORM_HEIGHT"); heightEnv != "" {
		if h, err := strconv.Atoi(heightEnv); err == nil && h > 0 {
			formHeight = h
		}
	}
	form = form.WithHeight(formHeight)

	// Enable accessibility mode for very constrained terminals when requested
	// This falls back to standard prompts which handle small windows better
	if os.Getenv("ACCESSIBLE") != "" || os.Getenv("CS_ACCESSIBLE") != "" {
		form = form.WithAccessible(true)
	}

	err := form.Run()
	if err != nil {
		// Handle user cancellation (Ctrl-C) silently
		if isHuhUserCancellation(err) {
			os.Exit(0)
		}
		return nil, err
	}

	// Convert boolean values back to strings
	result := make(map[string]string)
	for _, variable := range variables {
		if variable.Type == "boolean" {
			if *boolValues[variable.Name] {
				result[variable.Name] = "true"
			} else {
				result[variable.Name] = "false"
			}
		} else {
			result[variable.Name] = *valuePointers[variable.Name]
		}
	}

	return result, nil
}

// promptForVariable prompts for a single variable
func (p *Processor) promptForVariable(variable models.Variable) (string, error) {
	// Build prompt message
	message := variable.Name
	if variable.Description != "" {
		message = fmt.Sprintf("%s (%s)", variable.Name, variable.Description)
	}

	// Get direct terminal access to work around stdout redirection
	termIn, termOut, err := getTerminalIO()
	if err != nil {
		return "", fmt.Errorf("cannot access terminal: %w", err)
	}

	// Configure survey to use terminal directly
	stdio := survey.WithStdio(termIn, termOut, termOut)

	// Handle different variable types
	switch variable.Type {
	case "boolean":
		confirm := false
		prompt := &survey.Confirm{Message: message}
		err := survey.AskOne(prompt, &confirm, stdio)
		if confirm {
			return "true", err
		}
		return "false", err

	default:
		// Regular string input
		var value string
		prompt := &survey.Input{
			Message: message,
			Default: variable.DefaultValue,
		}

		// Add validation for enum types
		if variable.Validation != nil && len(variable.Validation.Enum) > 0 {
			selectPrompt := &survey.Select{
				Message: message,
				Options: variable.Validation.Enum,
				Default: variable.DefaultValue,
			}
			return value, survey.AskOne(selectPrompt, &value, stdio)
		}

		return value, survey.AskOne(prompt, &value, stdio)
	}
}

// executeCommand executes a shell command
func (p *Processor) executeCommand(command string) error {
	fmt.Fprintf(os.Stderr, "Executing: %s\n", command)

	// Split command into parts for proper execution
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = nil // Let output go to terminal
	cmd.Stderr = nil // Let errors go to terminal
	cmd.Stdin = nil  // Let input come from terminal

	return cmd.Run()
}
