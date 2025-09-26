package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/samling/command-snippets/internal/models"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [transforms|types|config]",
		Short: "Show configuration components",
		Long: `Show different configuration components like transform templates, variable types, and configuration summary.

Available subcommands:
  transforms  - Show all transform templates
  types       - Show all variable types  
  config      - Show configuration summary

Examples:
  cs show transforms    # Show all transform templates
  cs show types         # Show all variable types
  cs show config        # Show configuration overview`,
		Args: cobra.ExactArgs(1),
		RunE: runShow,
	}

	return cmd
}

func runShow(cmd *cobra.Command, args []string) error {
	subcommand := args[0]
	
	switch subcommand {
	case "transforms":
		return showTransforms()
	case "types":
		return showTypes()
	case "config":
		return showConfig()
	default:
		return fmt.Errorf("unknown subcommand: %s\nAvailable: transforms, types, config", subcommand)
	}
}

func showTransforms() error {
	if len(config.TransformTemplates) == 0 {
		fmt.Println("No transform templates defined.")
		return nil
	}

	fmt.Printf("Transform Templates:\n\n")

	// Get all transform template names and sort them alphabetically
	var names []string
	for name := range config.TransformTemplates {
		names = append(names, name)
	}
	sort.Strings(names)

	// Display each transform template
	for i, name := range names {
		if i > 0 {
			fmt.Println() // Add spacing between templates
		}
		
		template := config.TransformTemplates[name]
		fmt.Printf("%s:\n", name)
		
		if template.Description != "" {
			fmt.Printf("  Description: %s\n", template.Description)
		}
		
		if template.Transform != nil {
			displayTransform(template.Transform, "  ")
		}
	}

	return nil
}

func showTypes() error {
	if len(config.VariableTypes) == 0 {
		fmt.Println("No variable types defined.")
		return nil
	}

	fmt.Printf("Variable Types:\n\n")

	// Get all variable type names and sort them alphabetically
	var names []string
	for name := range config.VariableTypes {
		names = append(names, name)
	}
	sort.Strings(names)

	// Display each variable type
	for i, name := range names {
		if i > 0 {
			fmt.Println() // Add spacing between types
		}
		
		varType := config.VariableTypes[name]
		fmt.Printf("%s:\n", name)
		
		if varType.Description != "" {
			fmt.Printf("  Description: %s\n", varType.Description)
		}
		
		if varType.Default != "" {
			fmt.Printf("  Default: %s\n", varType.Default)
		}
		
		if varType.Validation != nil {
			fmt.Printf("  Validation:\n")
			displayValidation(varType.Validation, "    ")
		}
		
		if varType.Transform != nil {
			fmt.Printf("  Transform:\n")
			displayTransform(varType.Transform, "    ")
		}
	}

	return nil
}

func showConfig() error {
	fmt.Printf("Configuration Summary:\n\n")

	// Transform templates count
	fmt.Printf("Transform Templates: %d\n", len(config.TransformTemplates))
	if len(config.TransformTemplates) > 0 {
		var names []string
		for name := range config.TransformTemplates {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Printf("  - %s\n", strings.Join(names, "\n  - "))
	}
	
	fmt.Println()
	
	// Variable types count
	fmt.Printf("Variable Types: %d\n", len(config.VariableTypes))
	if len(config.VariableTypes) > 0 {
		var names []string
		for name := range config.VariableTypes {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Printf("  - %s\n", strings.Join(names, "\n  - "))
	}
	
	fmt.Println()
	
	// Snippets count
	fmt.Printf("Snippets: %d\n", len(config.Snippets))
	if len(config.Snippets) > 0 {
		var names []string
		for name := range config.Snippets {
			names = append(names, name)
		}
		sort.Strings(names)
		// Show first few, then count if there are many
		if len(names) <= 10 {
			fmt.Printf("  - %s\n", strings.Join(names, "\n  - "))
		} else {
			fmt.Printf("  - %s\n", strings.Join(names[:5], "\n  - "))
			fmt.Printf("  ... and %d more\n", len(names)-5)
		}
	}
	
	fmt.Println()
	
	// Settings
	fmt.Printf("Settings:\n")
	if len(config.Settings.AdditionalConfigs) > 0 {
		fmt.Printf("  Additional Configs: %s\n", strings.Join(config.Settings.AdditionalConfigs, ", "))
	}
	if config.Settings.Selector.Command != "" {
		fmt.Printf("  External Selector: %s %s\n", config.Settings.Selector.Command, config.Settings.Selector.Options)
	}
	fmt.Printf("  Interactive Settings: confirm_before_execute=%t, show_final_command=%t\n", 
		config.Settings.Interactive.ConfirmBeforeExecute, 
		config.Settings.Interactive.ShowFinalCommand)

	return nil
}

// displayTransform shows transform details with proper formatting
func displayTransform(transform *models.Transform, indent string) {
	if transform.EmptyValue != "" {
		fmt.Printf("%sEmpty Value: %s\n", indent, transform.EmptyValue)
	}
	
	if transform.ValuePattern != "" {
		// Handle multiline value patterns
		lines := strings.Split(strings.TrimSpace(transform.ValuePattern), "\n")
		if len(lines) == 1 {
			fmt.Printf("%sValue Pattern: %s\n", indent, lines[0])
		} else {
			fmt.Printf("%sValue Pattern: |\n", indent)
			for _, line := range lines {
				fmt.Printf("%s  %s\n", indent, line)
			}
		}
	}
	
	if transform.TrueValue != "" {
		fmt.Printf("%sTrue Value: %s\n", indent, transform.TrueValue)
	}
	
	if transform.FalseValue != "" {
		fmt.Printf("%sFalse Value: %s\n", indent, transform.FalseValue)
	}
	
	if transform.Compose != "" {
		// Handle multiline compose patterns
		lines := strings.Split(strings.TrimSpace(transform.Compose), "\n")
		if len(lines) == 1 {
			fmt.Printf("%sCompose: %s\n", indent, lines[0])
		} else {
			fmt.Printf("%sCompose: |\n", indent)
			for _, line := range lines {
				fmt.Printf("%s  %s\n", indent, line)
			}
		}
	}
}

// displayValidation shows validation rules with proper formatting
func displayValidation(validation *models.Validation, indent string) {
	if len(validation.Enum) > 0 {
		fmt.Printf("%sAllowed values: %s\n", indent, strings.Join(validation.Enum, ", "))
	}
	
	if len(validation.Range) == 2 {
		fmt.Printf("%sRange: %d - %d\n", indent, validation.Range[0], validation.Range[1])
	}
	
	if validation.Pattern != "" {
		fmt.Printf("%sPattern: %s\n", indent, validation.Pattern)
	}
}
