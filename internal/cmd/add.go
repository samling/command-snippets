package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/samling/command-snippets/internal/models"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new command template",
		Long: `Add a new command template interactively.

All variables must be explicitly defined in the template. You can use:
- Inline transforms for simple transformations
- Transform templates for reusable transformation logic

Examples:
  cs add                         # Interactive template creation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd()
		},
	}

	return cmd
}

func runAdd() error {
	snippet, err := promptForSnippet()
	if err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	// Add to config
	config.Snippets[snippet.Name] = *snippet

	// Save config
	if err := saveConfig(config, cfgFile); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Command template '%s' added successfully!\n", snippet.Name)
	return nil
}

func promptForSnippet() (*models.Snippet, error) {
	snippet := &models.Snippet{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Prompt for basic information
	questions := []*survey.Question{
		{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Template name:"},
			Validate: survey.Required,
		},
		{
			Name:   "description",
			Prompt: &survey.Input{Message: "Description:"},
		},
		{
			Name:     "command",
			Prompt:   &survey.Input{Message: "Command template (use <variable> syntax):"},
			Validate: survey.Required,
		},
		{
			Name:   "tags",
			Prompt: &survey.Input{Message: "Tags (comma-separated):"},
		},
	}

	answers := struct {
		Name        string
		Description string
		Command     string
		Tags        string
	}{}

	if err := survey.Ask(questions, &answers); err != nil {
		return nil, err
	}

	snippet.Name = answers.Name
	snippet.Description = answers.Description
	snippet.Command = answers.Command

	// Parse tags
	if answers.Tags != "" {
		tagList := strings.Split(answers.Tags, ",")
		for _, tag := range tagList {
			snippet.Tags = append(snippet.Tags, strings.TrimSpace(tag))
		}
	}

	// Extract variables from command template
	variables := extractVariablesFromCommand(answers.Command)

	// Prompt for variable configuration (all variables must be explicitly defined)
	for _, varName := range variables {
		variable, err := promptForVariable(varName)
		if err != nil {
			return nil, err
		}
		snippet.Variables = append(snippet.Variables, *variable)
	}

	snippet.ID = generateSnippetID(snippet.Name)
	return snippet, nil
}

func extractVariablesFromCommand(command string) []string {
	var variables []string
	words := strings.Fields(command)

	for _, word := range words {
		if strings.HasPrefix(word, "<") && strings.HasSuffix(word, ">") {
			varName := strings.Trim(word, "<>")
			if varName != "" {
				// Avoid duplicates
				found := false
				for _, existing := range variables {
					if existing == varName {
						found = true
						break
					}
				}
				if !found {
					variables = append(variables, varName)
				}
			}
		}
	}

	return variables
}

func promptForVariable(varName string) (*models.Variable, error) {
	fmt.Printf("\nConfiguring variable: %s\n", varName)

	variable := &models.Variable{
		Name: varName,
	}

	questions := []*survey.Question{
		{
			Name:   "description",
			Prompt: &survey.Input{Message: "Description:"},
		},
		{
			Name:   "default",
			Prompt: &survey.Input{Message: "Default value:"},
		},
		{
			Name:   "required",
			Prompt: &survey.Confirm{Message: "Required?", Default: false},
		},
	}

	answers := struct {
		Description string
		Default     string
		Required    bool
	}{}

	if err := survey.Ask(questions, &answers); err != nil {
		return nil, err
	}

	variable.Description = answers.Description
	variable.DefaultValue = answers.Default
	variable.Required = answers.Required

	// Ask about transform template or inline transform
	transformChoice := ""
	transformOptions := []string{"None", "Inline transform", "Transform template"}
	if err := survey.AskOne(&survey.Select{
		Message: "Transformation type:",
		Options: transformOptions,
		Default: "None",
	}, &transformChoice); err != nil {
		return nil, err
	}

	switch transformChoice {
	case "Transform template":
		// Show available transform templates
		if len(config.TransformTemplates) == 0 {
			fmt.Println("No transform templates available. Creating inline transform instead.")
			variable.Transform = promptForInlineTransform()
		} else {
			templates := make([]string, 0, len(config.TransformTemplates))
			for name := range config.TransformTemplates {
				templates = append(templates, name)
			}

			var selectedTemplate string
			if err := survey.AskOne(&survey.Select{
				Message: "Select transform template:",
				Options: templates,
			}, &selectedTemplate); err != nil {
				return nil, err
			}
			variable.TransformTemplate = selectedTemplate
		}

	case "Inline transform":
		variable.Transform = promptForInlineTransform()
	}

	return variable, nil
}

func promptForInlineTransform() *models.Transform {
	transform := &models.Transform{}

	transformQuestions := []*survey.Question{
		{
			Name:   "empty_value",
			Prompt: &survey.Input{Message: "Value when empty:"},
		},
		{
			Name:   "value_pattern",
			Prompt: &survey.Input{Message: "Pattern when has value (use {{.Value}}):"},
		},
	}

	transformAnswers := struct {
		EmptyValue   string
		ValuePattern string
	}{}

	if err := survey.Ask(transformQuestions, &transformAnswers); err != nil {
		return nil
	}

	transform.EmptyValue = transformAnswers.EmptyValue
	transform.ValuePattern = transformAnswers.ValuePattern

	return transform
}

func generateSnippetID(name string) string {
	// Simple ID generation - in production, you might want something more robust
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}
