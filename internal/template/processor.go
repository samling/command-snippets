package template

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/samling/command-snippets/internal/models"
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

		confirm, err := promptForConfirmation("Execute this command?")
		if err != nil {
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
	// Use Bubble Tea form for prompting
	return promptForVariablesWithBubbleTea(snippet, nil, p.config)
}

// promptForVariablesWithPresets interactively prompts for snippet variables, using preset values where available
func (p *Processor) promptForVariablesWithPresets(snippet *models.Snippet, presetValues map[string]string) (map[string]string, error) {
	// Use Bubble Tea form for prompting
	return promptForVariablesWithBubbleTea(snippet, presetValues, p.config)
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
