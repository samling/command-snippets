package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDescribeCmd(state *commandState) *cobra.Command {
	var id string
	cmd := &cobra.Command{Use: "describe [NAME]", Short: "Inspect a command, its inputs, source and persistent ID", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && id == "" {
			return fmt.Errorf("describe requires NAME or --id UUID")
		}
		entry, err := state.lookup(args, id)
		if err != nil {
			return err
		}
		data, err := yaml.Marshal(entry.Snippet)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Source: %s\n", entry.Source.DisplayPath); err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}}
	cmd.Flags().StringVar(&id, "id", "", "select a persisted UUID")
	return cmd
}
