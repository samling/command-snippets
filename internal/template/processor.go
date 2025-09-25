package template

import (
	"fmt"
	"os/exec"
	"strings"

	"tplkit/internal/models"

	"github.com/AlecAivazis/survey/v2"
)

// ExecutionMode defines how commands should be executed
type ExecutionMode int

const (
	PrintOnly     ExecutionMode = iota // Print command only (default)
	AutoExecute                        // Execute automatically without prompting
	PromptExecute                      // Prompt before executing (original behavior)
)

// Processor handles snippet template processing
type Processor struct {
	config *models.Config
}

// NewProcessor creates a new template processor
func NewProcessor(config *models.Config) *Processor {
	return &Processor{
		config: config,
	}
}

// ExecuteWithMode prompts for variables and handles execution based on specified mode
func (p *Processor) ExecuteWithMode(snippet *models.Snippet, mode ExecutionMode) error {
	values, err := p.promptForVariables(snippet)
	if err != nil {
		return err
	}

	command, err := snippet.ProcessTemplate(values, p.config)
	if err != nil {
		return err
	}

	// Handle execution based on mode
	switch mode {
	case PrintOnly:
		// Print just the raw command (perfect for piping)
		fmt.Println(command)
		return nil

	case AutoExecute:
		// Show command with prefix, then execute
		fmt.Printf("Command: %s\n", command)
		return p.executeCommand(command)

	case PromptExecute:
		// Show command with prefix, then ask for confirmation
		fmt.Printf("Command: %s\n", command)
		confirm := false
		prompt := &survey.Confirm{
			Message: "Execute this command?",
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
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
				return nil, err
			}

			// Validate the value (using config for type-based validation)
			if err := variable.ValidateWithConfig(value, p.config); err != nil {
				fmt.Printf("❌ %v\n", err)
				fmt.Println("Please try again.")
				continue // Reprompt for this variable
			}

			// Valid input - store it and move to next variable
			values[variable.Name] = value
			break
		}
	}

	return values, nil
}

// promptForVariable prompts for a single variable
func (p *Processor) promptForVariable(variable models.Variable) (string, error) {
	// Build prompt message
	message := variable.Name
	if variable.Description != "" {
		message = fmt.Sprintf("%s (%s)", variable.Name, variable.Description)
	}

	// Handle different variable types
	switch variable.Type {
	case "boolean":
		confirm := false
		prompt := &survey.Confirm{Message: message}
		err := survey.AskOne(prompt, &confirm)
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
			return value, survey.AskOne(selectPrompt, &value)
		}

		return value, survey.AskOne(prompt, &value)
	}
}

// executeCommand executes a shell command
func (p *Processor) executeCommand(command string) error {
	fmt.Printf("Executing: %s\n", command)

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
