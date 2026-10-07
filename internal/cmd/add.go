package cmd

import (
	"github.com/samling/command-snippets/internal/workspace"
	"github.com/spf13/cobra"
)

func newAddCmd(state *commandState) *cobra.Command {
	return &cobra.Command{Use: "add", Short: "Create a command using guided controls", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return state.interactive(cmd, workspace.Options{Start: "add"})
	}}
}
