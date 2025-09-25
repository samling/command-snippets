package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search command templates by name, description, or command",
		Long: `Search through your command templates using a query string.

The search looks through template names, descriptions, commands, and tags.

Examples:
  cs search kubectl              # Find templates containing "kubectl"
  cs search "get pods"           # Find templates with "get pods"
  cs search                      # Interactive search`,
		RunE: runSearch,
	}

	return cmd
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = strings.Join(args, " ")
	}

	if query == "" {
		// Interactive search could be implemented here
		fmt.Println("Usage: cs search <query>")
		return nil
	}

	matches := searchSnippets(query)

	if len(matches) == 0 {
		fmt.Printf("No command templates found matching '%s'\n", query)
		return nil
	}

	fmt.Printf("Found %d template(s) matching '%s':\n\n", len(matches), query)

	for _, name := range matches {
		snippet := config.Snippets[name]

		fmt.Printf("• %s", name)

		if snippet.Description != "" {
			fmt.Printf(" - %s", snippet.Description)
		}

		if len(snippet.Tags) > 0 {
			fmt.Printf(" [%s]", strings.Join(snippet.Tags, ", "))
		}

		fmt.Printf("\n  Command: %s\n\n", snippet.Command)
	}

	return nil
}

func searchSnippets(query string) []string {
	var matches []string
	queryLower := strings.ToLower(query)

	for name, snippet := range config.Snippets {
		// Search in name
		if strings.Contains(strings.ToLower(name), queryLower) {
			matches = append(matches, name)
			continue
		}

		// Search in description
		if strings.Contains(strings.ToLower(snippet.Description), queryLower) {
			matches = append(matches, name)
			continue
		}

		// Search in command
		if strings.Contains(strings.ToLower(snippet.Command), queryLower) {
			matches = append(matches, name)
			continue
		}

		// Search in tags
		for _, tag := range snippet.Tags {
			if strings.Contains(strings.ToLower(tag), queryLower) {
				matches = append(matches, name)
				break
			}
		}
	}

	return matches
}
