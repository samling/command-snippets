package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var tags []string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available command templates",
		Long: `List all available command templates with their descriptions and tags.

Examples:
  cs list                    # List all templates
  cs list --tags k8s         # List templates with 'k8s' tag
  cs list --verbose          # Show detailed information`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(tags, verbose)
		},
	}

	cmd.Flags().StringSliceVarP(&tags, "tags", "t", []string{}, "Filter by tags")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed information")

	return cmd
}

func runList(filterTags []string, verbose bool) error {
	if len(config.Snippets) == 0 {
		fmt.Println("No command templates found. Use 'cs add' to create your first template.")
		return nil
	}

	fmt.Printf("Available command templates:\n\n")

	for name, snippet := range config.Snippets {
		// Filter by tags if specified
		if len(filterTags) > 0 && !hasAnyTag(snippet.Tags, filterTags) {
			continue
		}

		// Basic display
		fmt.Printf("• %s", name)

		if snippet.Description != "" {
			fmt.Printf(" - %s", snippet.Description)
		}

		// Show tags
		if len(snippet.Tags) > 0 {
			fmt.Printf(" [%s]", strings.Join(snippet.Tags, ", "))
		}

		fmt.Println()

		// Verbose mode shows more details
		if verbose {
			fmt.Printf("  Command: %s\n", snippet.Command)

			if len(snippet.Variables) > 0 {
				fmt.Printf("  Variables:\n")
				for _, variable := range snippet.Variables {
					fmt.Printf("    - %s", variable.Name)
					if variable.Description != "" {
						fmt.Printf(" (%s)", variable.Description)
					}
					if variable.Required {
						fmt.Printf(" *required*")
					}
					if variable.DefaultValue != "" {
						fmt.Printf(" [default: %s]", variable.DefaultValue)
					}
					if variable.TransformTemplate != "" {
						fmt.Printf(" [transform: %s]", variable.TransformTemplate)
					} else if variable.Transform != nil {
						fmt.Printf(" [inline transform]")
					}
					fmt.Println()
				}
			}
			fmt.Println()
		}
	}

	return nil
}

// hasAnyTag checks if any of the filterTags exist in the snippet tags
func hasAnyTag(snippetTags, filterTags []string) bool {
	for _, filterTag := range filterTags {
		for _, snippetTag := range snippetTags {
			if strings.EqualFold(snippetTag, filterTag) {
				return true
			}
		}
	}
	return false
}
