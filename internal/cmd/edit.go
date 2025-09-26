package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/samling/command-snippets/internal/models"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [template-name]",
		Short: "Edit an existing command template or open config file",
		Long: `Edit a command template or configuration file in your default editor.

Examples:
  cs edit kubectl-get-pods       # Edit specific template
  cs edit --config               # Edit configuration file`,
		RunE: runEdit,
	}

	cmd.Flags().Bool("config", false, "Edit the configuration file")

	return cmd
}

func runEdit(cmd *cobra.Command, args []string) error {
	editConfig, _ := cmd.Flags().GetBool("config")

	if editConfig {
		return editConfigFile()
	}

	if len(args) == 0 {
		return fmt.Errorf("please specify a template name to edit, or use --config to edit the configuration file")
	}

	snippetName := args[0]
	snippet, exists := config.Snippets[snippetName]
	if !exists {
		return fmt.Errorf("template '%s' not found", snippetName)
	}

	return editSnippet(snippetName, &snippet)
}

func editConfigFile() error {
	editor := getEditor()
	cmd := exec.Command(editor, cfgFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func editSnippet(name string, snippet *models.Snippet) error {
	// Create a temporary file with the snippet YAML
	tempFile, err := os.CreateTemp("", fmt.Sprintf("cs-edit-%s-*.yaml", name))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write current snippet to temp file
	data, err := yaml.Marshal(snippet)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()

	// Open editor
	editor := getEditor()
	cmd := exec.Command(editor, tempFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Read back the edited content
	editedData, err := os.ReadFile(tempFile.Name())
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	// Parse the edited YAML
	var editedSnippet models.Snippet
	if err := yaml.Unmarshal(editedData, &editedSnippet); err != nil {
		return fmt.Errorf("invalid YAML in edited template: %w", err)
	}

	// Update the snippet in config
	config.Snippets[name] = editedSnippet

	// Save config
	if err := saveConfig(config, cfgFile); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Command template '%s' updated successfully!\n", name)
	return nil
}

func getEditor() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	return editor
}
