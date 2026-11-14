package cmd

import (
	"fmt"

	"github.com/samling/command-snippets/internal/templating"
	"github.com/spf13/cobra"
)

func newRenderCmd(state *commandState) *cobra.Command {
	var id string
	var sets []string
	cmd := &cobra.Command{Use: "render [NAME]", Short: "Render a validated command without a terminal or execution", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && id == "" {
			return fmt.Errorf("render requires NAME or --id UUID")
		}
		entry, err := state.lookup(args, id)
		if err != nil {
			return err
		}
		values, err := templating.Presets(entry.Snippet, sets)
		if err != nil {
			return err
		}
		command, err := entry.Template.Render(values)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), command)
		return err
	}}
	cmd.Flags().StringVar(&id, "id", "", "select a persisted UUID")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "input name=value; repeat for repeat-input items")
	return cmd
}
