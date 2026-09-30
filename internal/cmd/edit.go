package cmd

import (
	"github.com/samling/command-snippets/internal/workspace"
	"github.com/spf13/cobra"
)

func newEditCmd(state *commandState) *cobra.Command {
	var id string
	cmd := &cobra.Command{Use: "edit [NAME]", Short: "Edit a command in its owning source", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		options := workspace.Options{Start: "edit"}
		if len(args) > 0 || id != "" {
			entry, err := state.lookup(args, id)
			if err != nil {
				return err
			}
			options.Entry = entry
		}
		return state.interactive(cmd, options)
	}}
	cmd.Flags().StringVar(&id, "id", "", "select a persisted UUID")
	return cmd
}
