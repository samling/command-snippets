package template

import (
	"fmt"
	"os"
	"os/exec"

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

// ProcessSnippet processes a snippet with given values (non-interactive)
func (p *Processor) ProcessSnippet(snippet *models.Snippet, values map[string]string) (string, error) {
	return snippet.ProcessTemplate(values, p.config)
}

// promptForVariablesWithPresets interactively prompts for snippet variables, using preset values where available
func (p *Processor) promptForVariablesWithPresets(snippet *models.Snippet, presetValues map[string]string) (map[string]string, error) {
	// Use Bubble Tea form for prompting
	return promptForVariablesWithBubbleTea(snippet, presetValues, p.config)
}

// executeCommand runs the command through the user's shell so quoting,
// pipes, redirection, and `&&` chains behave as a user would expect.
func (p *Processor) executeCommand(command string) error {
	fmt.Fprintf(os.Stderr, "Executing: %s\n", command)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
