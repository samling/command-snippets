package cmd

import (
	"fmt"
	"strings"

	"github.com/samling/command-snippets/internal/models"
	"github.com/spf13/cobra"
)

func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe [template-name]",
		Short: "Show detailed information about a command template",
		Long: `Show detailed information about a command template including variables, validation, and usage.

This command displays:
- Template description and command pattern
- All variables with their types, validation rules, and defaults
- Tags for organization
- Transform templates used

Examples:
  cs describe kubectl-get-pods     # Show details for specific template
  cs describe docker-run          # Show variables and validation rules`,
		Args: cobra.ExactArgs(1),
		RunE: runDescribe,
	}

	return cmd
}

func runDescribe(cmd *cobra.Command, args []string) error {
	snippetName := args[0]

	snippet, err := getSnippet(snippetName)
	if err != nil {
		return err
	}

	// Display snippet information
	fmt.Printf("Name: %s\n", snippetName)

	if snippet.Description != "" {
		fmt.Printf("Description: %s\n", snippet.Description)
	}

	fmt.Printf("\nCommand Template:\n")
	fmt.Printf("  %s\n", snippet.Command)

	// Show tags if present
	if len(snippet.Tags) > 0 {
		fmt.Printf("\nTags: %s\n", strings.Join(snippet.Tags, ", "))
	}

	// Show variables
	if len(snippet.Variables) > 0 {
		fmt.Printf("\nVariables:\n")
		for _, variable := range snippet.Variables {
			displayVariable(variable)
		}
	} else {
		fmt.Printf("\nNo variables defined.\n")
	}

	return nil
}

func displayVariable(variable models.Variable) {
	fmt.Printf("\n  %s:\n", variable.Name)

	if variable.Description != "" {
		fmt.Printf("    Description: %s\n", variable.Description)
	}
	if variable.Type != "" {
		fmt.Printf("    Type: %s\n", variable.Type)
	}
	if variable.DefaultValue != "" {
		fmt.Printf("    Default: %s\n", variable.DefaultValue)
	}
	if variable.Required {
		fmt.Printf("    Required: true\n")
	}
	if variable.Computed {
		fmt.Printf("    Computed: true\n")
	}

	if variable.TransformTemplate != "" {
		fmt.Printf("    Transform Template: %s\n", variable.TransformTemplate)
		if t, exists := config.TransformTemplates[variable.TransformTemplate]; exists {
			if t.Description != "" {
				fmt.Printf("      Description: %s\n", t.Description)
			}
			if t.Transform != nil {
				displayTransform(t.Transform, "      ")
			}
		}
	}

	if variable.Transform != nil {
		fmt.Printf("    Transform:\n")
		displayTransform(variable.Transform, "      ")
	}

	if variable.Validation != nil {
		fmt.Printf("    Validation:\n")
		displayValidation(variable.Validation, "      ")
	}

	if variable.Type != "" {
		if varType, exists := config.VariableTypes[variable.Type]; exists {
			if varType.Description != "" {
				fmt.Printf("    Type Description: %s\n", varType.Description)
			}
			if varType.Default != "" && variable.DefaultValue == "" {
				fmt.Printf("    Type Default: %s\n", varType.Default)
			}
			if varType.Validation != nil {
				fmt.Printf("    Type Validation:\n")
				displayValidation(varType.Validation, "      ")
			}
		}
	}
}
