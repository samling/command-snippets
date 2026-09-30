package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/workspace"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

var cfgFile string
var version = "dev"

type commandState struct {
	lib     *library.Library
	noColor bool
}

func Execute() error { return NewRoot().Execute() }
func NewRoot() *cobra.Command {
	state := &commandState{}
	var generate bool
	root := &cobra.Command{Use: "cs", Short: "A searchable command library for your shell", Version: version, SilenceUsage: true, SilenceErrors: true}
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $XDG_CONFIG_HOME/cs/config.yaml or $HOME/.config/cs/config.yaml)")
	root.PersistentFlags().BoolVar(&state.noColor, "no-color", false, "disable terminal colors")
	root.Flags().BoolVar(&generate, "generate-config", false, "print the default configuration")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "init" || generate {
			return nil
		}
		path := cfgFile
		if path == "" {
			var err error
			path, err = defaultConfigPath()
			if err != nil {
				return err
			}
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		state.lib = library.Load(path, cwd)
		for _, warning := range state.lib.Warnings {
			fmt.Fprintln(cmd.ErrOrStderr(), "Warning:", warning)
		}
		return nil
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if generate {
			data, err := defaultConfigYAML()
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		}
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q", args[0])
		}
		return state.interactive(cmd, workspace.Options{})
	}
	root.AddCommand(newInitCmd(), newAddCmd(state), newEditCmd(state), newExecCmd(state), newRenderCmd(state), newListCmd(state), newDescribeCmd(state), &cobra.Command{Use: "validate", Short: "Validate every source without writing", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { return state.lib.Error() }})
	return root
}
func (s *commandState) interactive(cmd *cobra.Command, options workspace.Options) error {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return fmt.Errorf("interactive CS needs a terminal on stderr; use cs render NAME --set name=value")
	}
	options.NoColor = s.noColor
	command, err := workspace.Run(s.lib, options)
	if err != nil {
		return err
	}
	if command != "" {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), command)
	}
	return err
}
func (s *commandState) lookup(args []string, id string) (*library.Entry, error) {
	if err := s.lib.Error(); err != nil {
		return nil, err
	}
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	return s.lib.Find(name, id)
}
func defaultConfigPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); strings.TrimSpace(xdg) != "" {
		return filepath.Join(xdg, "cs", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cs", "config.yaml"), nil
}
func missingConfigError(path string) error {
	return fmt.Errorf("config file not found at %s; run cs init to create one", path)
}
func createDefaultConfig() *models.Config {
	settings := models.DefaultSettings()
	settings.Sources = []string{"snippets/*.yaml"}
	settings.DefaultSource = "snippets/custom.yaml"
	return &models.Config{Settings: settings, Snippets: []models.Snippet{}}
}
func defaultConfigYAML() ([]byte, error) { return yaml.Marshal(createDefaultConfig()) }
