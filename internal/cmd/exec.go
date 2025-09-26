package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"

	"github.com/samling/command-snippets/internal/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [template-name]",
		Short: "Execute a command template with variable substitution",
		Long: `Execute a command template with interactive variable prompting.

By default, the command will be printed for copying/piping. Use flags to change behavior.

If no template name is provided, you'll be prompted to select from available templates.

Examples:
  cs exec kubectl-get-pods              # Print command only (default)
  cs exec kubectl-get-pods --run        # Execute automatically
  cs exec kubectl-get-pods --prompt     # Prompt before executing`,
		RunE: runExec,
	}

	// Add execution mode flags
	cmd.Flags().Bool("run", false, "Automatically execute the command without prompting")
	cmd.Flags().Bool("prompt", false, "Prompt before executing the command")
	cmd.Flags().Bool("no-selector", false, "Use internal selector instead of configured external selector")

	return cmd
}

func runExec(cmd *cobra.Command, args []string) error {
	processor := template.NewProcessor(config)

	var snippetName string

	// If snippet name provided as argument
	if len(args) > 0 {
		snippetName = args[0]
	} else {
		// Interactive snippet selection
		noSelector, _ := cmd.Flags().GetBool("no-selector")
		var err error
		snippetName, err = selectSnippet(noSelector)
		if err != nil {
			// Handle user cancellation silently
			if isUserCancellation(err) {
				os.Exit(0)
			}
			return fmt.Errorf("failed to select template: %w", err)
		}
	}

	// Find the snippet
	snippet, exists := config.Snippets[snippetName]
	if !exists {
		return fmt.Errorf("template '%s' not found", snippetName)
	}

	// Get execution mode flags
	runFlag, _ := cmd.Flags().GetBool("run")
	promptFlag, _ := cmd.Flags().GetBool("prompt")

	// Validate flags (mutually exclusive)
	if runFlag && promptFlag {
		return fmt.Errorf("--run and --prompt flags are mutually exclusive")
	}

	// Determine execution mode
	var execMode template.ExecutionMode
	switch {
	case runFlag:
		execMode = template.AutoExecute
	case promptFlag:
		execMode = template.PromptExecute
	default:
		execMode = template.PrintOnly
	}

	// Execute with specified mode
	return processor.ExecuteWithMode(&snippet, execMode)
}

// selectSnippet shows an interactive snippet selector
func selectSnippet(forceInternal bool) (string, error) {
	if len(config.Snippets) == 0 {
		return "", fmt.Errorf("no templates found")
	}

	// Build options for selection
	var options []string
	snippetMap := make(map[string]string)

	for name, snippet := range config.Snippets {
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

	// Sort options alphabetically for consistent ordering
	sort.Strings(options)

	// Try external selector first (if configured and not forced to use internal)
	if !forceInternal {
		selected, err := tryExternalSelector(options, snippetMap)
		if err == nil {
			return selected, nil
		}

		// Check if user cancelled (don't fallback for cancellation)
		if isUserCancellation(err) {
			return "", err
		}

		// For other errors, we'll fall back to internal selector
	}

	// Fall back to internal selector
	var selected string
	prompt := &survey.Select{
		Message: "Select a template to execute:",
		Options: options,
	}

	// Get direct terminal access to work around stdout redirection
	termIn, termOut, err := getTerminalIO()
	if err != nil {
		return "", fmt.Errorf("cannot access terminal: %w", err)
	}

	stdio := survey.WithStdio(termIn, termOut, termOut)

	if err := survey.AskOne(prompt, &selected, stdio); err != nil {
		// Handle survey interrupts and cancellations as user cancellation
		if isSurveyUserCancellation(err) {
			os.Exit(0)
		}
		return "", &UserCancellationError{"user cancelled selection"}
	}

	return snippetMap[selected], nil
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

// tryExternalSelector attempts to use configured external selector (like fzf)
func tryExternalSelector(options []string, snippetMap map[string]string) (string, error) {
	// Check if external selector is configured
	selectorCmd := config.Settings.Selector.Command
	if selectorCmd == "" {
		return "", fmt.Errorf("no external selector configured")
	}

	// Check if selector command is available
	if _, err := exec.LookPath(selectorCmd); err != nil {
		return "", fmt.Errorf("selector command '%s' not found: %w", selectorCmd, err)
	}

	// Prepare input for selector (one option per line)
	input := strings.Join(options, "\n")

	// Build command with options
	var cmdArgs []string
	if config.Settings.Selector.Options != "" {
		// Parse options string into individual arguments
		cmdArgs = strings.Fields(config.Settings.Selector.Options)
	}

	// Create and run the selector command
	cmd := exec.Command(selectorCmd, cmdArgs...)
	cmd.Stdin = strings.NewReader(input)

	var output bytes.Buffer
	cmd.Stdout = &output

	// Run the command
	if err := cmd.Run(); err != nil {
		// Check if this looks like a user cancellation
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode := status.ExitStatus()
				// Common exit codes for user cancellation:
				// 130 = Ctrl+C (SIGINT)
				// 1 = general cancellation in many tools
				if exitCode == 130 || exitCode == 1 {
					return "", &UserCancellationError{"user cancelled selection"}
				}
			}
		}
		return "", fmt.Errorf("selector command failed: %w", err)
	}

	// Parse the selected option
	selected := strings.TrimSpace(output.String())
	if selected == "" {
		return "", &UserCancellationError{"no selection made"}
	}

	// Look up the actual snippet name
	if snippetName, exists := snippetMap[selected]; exists {
		return snippetName, nil
	}

	return "", fmt.Errorf("selected option not found: %s", selected)
}

// UserCancellationError indicates the user cancelled the operation
type UserCancellationError struct {
	Message string
}

func (e *UserCancellationError) Error() string {
	return e.Message
}

// isUserCancellation checks if an error represents user cancellation
func isUserCancellation(err error) bool {
	_, ok := err.(*UserCancellationError)
	return ok
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
