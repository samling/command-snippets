package cmd

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/samling/command-snippets/internal/models"
)

// getSnippet looks up a snippet by name in the loaded config.
func getSnippet(name string) (models.Snippet, error) {
	snippet, exists := config.Snippets[name]
	if !exists {
		return models.Snippet{}, fmt.Errorf("template '%s' not found", name)
	}
	return snippet, nil
}

// snippetSummary renders "name - description [tag1, tag2]" suitable for
// list output, search results, and selector menus. Description and tags
// are omitted when empty.
func snippetSummary(name string, s *models.Snippet) string {
	var b strings.Builder
	b.WriteString(name)
	if s.Description != "" {
		b.WriteString(" - ")
		b.WriteString(s.Description)
	}
	if len(s.Tags) > 0 {
		b.WriteString(" [")
		b.WriteString(strings.Join(s.Tags, ", "))
		b.WriteString("]")
	}
	return b.String()
}

// buildSnippetOptions returns the snippet display strings (alphabetical) and
// the reverse lookup from display string back to snippet name. Used by both
// the external (fzf) and internal selectors.
func buildSnippetOptions(snippets map[string]*models.Snippet) (options []string, byDisplay map[string]string) {
	byDisplay = make(map[string]string, len(snippets))
	options = make([]string, 0, len(snippets))
	for _, name := range slices.Sorted(maps.Keys(snippets)) {
		display := snippetSummary(name, snippets[name])
		options = append(options, display)
		byDisplay[display] = name
	}
	return options, byDisplay
}
