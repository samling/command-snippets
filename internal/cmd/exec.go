package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/template"

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
  cs exec kubectl-get-pods --prompt     # Prompt before executing
  cs exec kubectl-get-pods --set namespace=kube-system  # Pre-set variables
  cs exec docker-run --set port=8080 --set image=nginx  # Multiple variables`,
		RunE: runExec,
	}

	// Add execution mode flags
	cmd.Flags().Bool("run", false, "Automatically execute the command without prompting")
	cmd.Flags().Bool("prompt", false, "Prompt before executing the command")
	cmd.Flags().Bool("no-selector", false, "Use internal selector instead of configured external selector")
	cmd.Flags().StringArray("set", []string{}, "Set variable values (format: key=value)")

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

	// Parse --set values
	setValues, _ := cmd.Flags().GetStringArray("set")

	presetValues, err := parseSetValues(setValues)
	if err != nil {
		return fmt.Errorf("invalid --set format: %w", err)
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
	return processor.ExecuteWithModeAndPresets(&snippet, execMode, presetValues)
}

// selectSnippet shows an interactive snippet selector
func selectSnippet(forceInternal bool) (string, error) {
	if len(config.Snippets) == 0 {
		return "", fmt.Errorf("no templates found")
	}

	// Build snippets map with pointers
	snippetsMap := make(map[string]*models.Snippet)
	for name, snippet := range config.Snippets {
		s := snippet // Create a copy to get a pointer
		snippetsMap[name] = &s
	}

	// Build options for external selector
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

	// Use Bubble Tea selector
	return selectSnippetWithBubbleTea(snippetsMap)
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

// parseSetValues parses --set values into a map
func parseSetValues(setValues []string) (map[string]string, error) {
	result := make(map[string]string)

	// Parse --set values
	for _, setValue := range setValues {
		key, value, err := parseKeyValue(setValue)
		if err != nil {
			return nil, fmt.Errorf("--set %s: %w", setValue, err)
		}
		result[key] = value
	}

	return result, nil
}

// parseKeyValue parses a key=value string
func parseKeyValue(input string) (string, string, error) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected format key=value, got: %s", input)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	if key == "" {
		return "", "", fmt.Errorf("key cannot be empty")
	}

	return key, value, nil
}
