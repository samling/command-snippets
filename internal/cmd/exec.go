package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	forms "github.com/samling/command-snippets/internal/template"
	"github.com/samling/command-snippets/internal/templating"
	"github.com/samling/command-snippets/internal/workspace"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newExecCmd(state *commandState) *cobra.Command {
	var id string
	var sets []string
	var run, prompt bool
	cmd := &cobra.Command{Use: "exec [NAME]", Short: "Fill a command and print it; execution is explicit only", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if run && prompt {
			return fmt.Errorf("--run and --prompt are mutually exclusive")
		}
		var command string
		var err error
		if len(args) == 0 && id == "" {
			if len(sets) > 0 {
				return fmt.Errorf("--set requires NAME or --id")
			}
			if !term.IsTerminal(int(os.Stderr.Fd())) {
				return fmt.Errorf("interactive CS needs a terminal; use render NAME")
			}
			command, err = workspace.Run(state.lib, workspace.Options{NoColor: state.noColor})
		} else {
			entry, lookupErr := state.lookup(args, id)
			if lookupErr != nil {
				return lookupErr
			}
			values, presetErr := templating.Presets(entry.Snippet, sets)
			if presetErr != nil {
				return presetErr
			}
			result := entry.Template.Preview(values)
			complete := result.Valid()
			for _, in := range entry.Snippet.Inputs {
				if result.Visible[in.Name] {
					if _, set := values[in.Name]; !set {
						complete = false
					}
				}
			}
			if complete {
				command = result.Command
			} else {
				if !term.IsTerminal(int(os.Stderr.Fd())) {
					return fmt.Errorf("incomplete inputs need a terminal; use render NAME --set name=value")
				}
				command, err = workspace.Run(state.lib, workspace.Options{Start: "inputs", Entry: entry, Presets: values, NoColor: state.noColor})
			}
		}
		if err != nil {
			return err
		}
		if command == "" {
			return nil
		}
		if prompt {
			if !term.IsTerminal(int(os.Stderr.Fd())) {
				return fmt.Errorf("--prompt needs a terminal")
			}
			confirmed, err := confirmExecution(command)
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
		}
		if run || prompt {
			shell := "sh"
			arguments := []string{"-c", command}
			if runtime.GOOS == "windows" {
				shell = "cmd.exe"
				arguments = []string{"/C", command}
			}
			child := exec.Command(shell, arguments...)
			child.Stdin = os.Stdin
			child.Stdout = cmd.OutOrStdout()
			child.Stderr = cmd.ErrOrStderr()
			return child.Run()
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), command)
		return err
	}}
	cmd.Flags().StringVar(&id, "id", "", "select a persisted UUID")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "input name=value; repeat to append list items")
	cmd.Flags().BoolVar(&run, "run", false, "explicitly execute the completed command")
	cmd.Flags().BoolVar(&prompt, "prompt", false, "ask before executing the completed command")
	return cmd
}

type executionConfirmation struct {
	command   string
	confirmed bool
}

func (m executionConfirmation) Init() tea.Cmd { return nil }
func (m executionConfirmation) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "Y":
			m.confirmed = true
			return m, tea.Quit
		case "n", "N", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m executionConfirmation) View() string {
	return "Execute this command?\n" + forms.Safe(m.command) + "\ny: execute  n/Esc: cancel"
}
func confirmExecution(command string) (bool, error) {
	program := tea.NewProgram(executionConfirmation{command: command}, tea.WithInputTTY(), tea.WithOutput(os.Stderr))
	result, err := program.Run()
	if err != nil {
		return false, err
	}
	return result.(executionConfirmation).confirmed, nil
}
