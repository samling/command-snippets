package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/samling/command-snippets/internal/search"
	"github.com/spf13/cobra"
)

func newListCmd(state *commandState) *cobra.Command {
	var query string
	var tags []string
	var jsonOutput bool
	cmd := &cobra.Command{Use: "list", Short: "Search friendly command names and intersect tags", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := state.lib.Error(); err != nil {
			return err
		}
		matches := search.New(state.lib.Entries).Query(query, tags, false)
		if jsonOutput {
			type row struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				Tags        []string `json:"tags"`
				Source      string   `json:"source"`
				ID          *string  `json:"id"`
			}
			rows := []row{}
			for _, match := range matches {
				entry := match.Entry
				var id *string
				if entry.Snippet.ID != "" {
					value := entry.Snippet.ID
					id = &value
				}
				rows = append(rows, row{entry.Snippet.Name, entry.Snippet.Description, entry.Snippet.Tags, entry.Source.DisplayPath, id})
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
		}
		for _, match := range matches {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), match.Entry.Snippet.Name); err != nil {
				return err
			}
		}
		return nil
	}}
	cmd.Flags().StringVar(&query, "query", "", "fuzzy search across command fields")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "required tag; repeated tags intersect")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "include descriptions, sources and explicit IDs as JSON")
	return cmd
}
