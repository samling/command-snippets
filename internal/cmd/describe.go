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

	// Find the snippet
	snippet, exists := config.Snippets[snippetName]
	if !exists {
		return fmt.Errorf("template '%s' not found", snippetName)
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
		if variable.Transform != nil && variable.Transform.Compose != "" {
			// Format multiline compose template with proper indentation
			composeLines := strings.Split(strings.TrimSpace(variable.Transform.Compose), "\n")
			if len(composeLines) == 1 {
				// Single line
				fmt.Printf("    Compose: %s\n", composeLines[0])
			} else {
				// Multi-line - show as a block
				fmt.Printf("    Compose: |\n")
				for _, line := range composeLines {
					fmt.Printf("      %s\n", line)
				}
			}
		}
	}

	if variable.TransformTemplate != "" {
		fmt.Printf("    Transform Template: %s\n", variable.TransformTemplate)

		// Show transform template details if available
		if template, exists := config.TransformTemplates[variable.TransformTemplate]; exists {
			if template.Description != "" {
				fmt.Printf("      Description: %s\n", template.Description)
			}
		}
	}

	// Show inline transform details
	if variable.Transform != nil {
		if variable.Transform.EmptyValue != "" {
			fmt.Printf("    Empty Value: %s\n", variable.Transform.EmptyValue)
		}
		if variable.Transform.ValuePattern != "" {
			fmt.Printf("    Value Pattern: %s\n", variable.Transform.ValuePattern)
		}
		if variable.Transform.TrueValue != "" {
			fmt.Printf("    True Value: %s\n", variable.Transform.TrueValue)
		}
		if variable.Transform.FalseValue != "" {
			fmt.Printf("    False Value: %s\n", variable.Transform.FalseValue)
		}
	}

	// Show validation rules
	if variable.Validation != nil {
		fmt.Printf("    Validation:\n")
		if len(variable.Validation.Enum) > 0 {
			fmt.Printf("      Allowed values: %s\n", strings.Join(variable.Validation.Enum, ", "))
		}
		if len(variable.Validation.Range) == 2 {
			fmt.Printf("      Range: %d - %d\n", variable.Validation.Range[0], variable.Validation.Range[1])
		}
		if variable.Validation.Pattern != "" {
			fmt.Printf("      Pattern: %s\n", variable.Validation.Pattern)
		}
	}

	// Show type-based validation from config
	if variable.Type != "" && config != nil {
		if varType, exists := config.VariableTypes[variable.Type]; exists {
			if varType.Description != "" {
				fmt.Printf("    Type Description: %s\n", varType.Description)
			}
			if varType.Default != "" && variable.DefaultValue == "" {
				fmt.Printf("    Type Default: %s\n", varType.Default)
			}
			if varType.Validation != nil {
				fmt.Printf("    Type Validation:\n")
				if len(varType.Validation.Enum) > 0 {
					fmt.Printf("      Allowed values: %s\n", strings.Join(varType.Validation.Enum, ", "))
				}
				if len(varType.Validation.Range) == 2 {
					fmt.Printf("      Range: %d - %d\n", varType.Validation.Range[0], varType.Validation.Range[1])
				}
				if varType.Validation.Pattern != "" {
					fmt.Printf("      Pattern: %s\n", varType.Validation.Pattern)
				}
			}
		}
	}
}
