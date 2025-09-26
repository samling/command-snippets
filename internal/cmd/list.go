package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/samling/command-snippets/internal/models"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var tags []string
	var verbose bool
	var showLocal bool
	var showGlobal bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available command templates",
		Long: `List all available command templates with their descriptions and tags.

Examples:
  cs list                    # List all templates (grouped by source)
  cs list --local            # Show only local (project-specific) templates
  cs list --global           # Show only global templates
  cs list --tags k8s         # List templates with 'k8s' tag
  cs list --verbose          # Show detailed information`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(tags, verbose, showLocal, showGlobal)
		},
	}

	cmd.Flags().StringSliceVarP(&tags, "tags", "t", []string{}, "Filter by tags")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed information")
	cmd.Flags().BoolVar(&showLocal, "local", false, "Show only local (project-specific) templates")
	cmd.Flags().BoolVar(&showGlobal, "global", false, "Show only global templates")

	return cmd
}

func runList(filterTags []string, verbose bool, showLocal bool, showGlobal bool) error {
	if len(config.Snippets) == 0 {
		fmt.Println("No command templates found. Use 'cs add' to create your first template.")
		return nil
	}

	// Handle mutually exclusive flags - if both are set, treat as if neither is set
	if showLocal && showGlobal {
		showLocal = false
		showGlobal = false
	}

	// Separate snippets by source
	globalSnippets := make(map[string]models.Snippet)
	localSnippets := make(map[string]models.Snippet)

	for name, snippet := range config.Snippets {
		// Filter by tags if specified
		if len(filterTags) > 0 && !hasAnyTag(snippet.Tags, filterTags) {
			continue
		}

		// Filter by source flags
		if showLocal && snippet.Source != models.SourceLocal {
			continue
		}
		if showGlobal && snippet.Source != models.SourceGlobal {
			continue
		}

		if snippet.Source == models.SourceLocal {
			localSnippets[name] = snippet
		} else {
			globalSnippets[name] = snippet
		}
	}

	// Check if we have any snippets to show
	totalSnippets := len(localSnippets) + len(globalSnippets)
	if totalSnippets == 0 {
		if showLocal {
			fmt.Println("No local (project-specific) templates found.")
		} else if showGlobal {
			fmt.Println("No global templates found.")
		} else if len(filterTags) > 0 {
			fmt.Printf("No templates found matching tags: %s\n", strings.Join(filterTags, ", "))
		} else {
			fmt.Println("No templates found.")
		}
		return nil
	}

	// Display local snippets first if any exist and we're not filtering for global only
	if len(localSnippets) > 0 && !showGlobal {
		if !showLocal {
			// Only show section header if we're showing both types
			fmt.Printf("Local (project-specific) templates:\n\n")
		}
		displaySnippetGroup(localSnippets, verbose)
	}

	// Display global snippets if any exist and we're not filtering for local only
	if len(globalSnippets) > 0 && !showLocal {
		// Add spacing if we showed local snippets
		if len(localSnippets) > 0 && !showGlobal {
			fmt.Println()
		}
		if !showGlobal {
			// Only show section header if we're showing both types
			fmt.Printf("Global templates:\n\n")
		}
		displaySnippetGroup(globalSnippets, verbose)
	}

	return nil
}

func displaySnippetGroup(snippets map[string]models.Snippet, verbose bool) {
	// Get all snippet names and sort them alphabetically
	var names []string
	for name := range snippets {
		names = append(names, name)
	}
	sort.Strings(names)

	// Iterate through sorted names
	for _, name := range names {
		snippet := snippets[name]

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
