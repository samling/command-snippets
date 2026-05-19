package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/samling/command-snippets/internal/defaults"

	"github.com/spf13/cobra"
)

type initOptions struct {
	Force   bool
	Missing bool
}

type initResult struct {
	Created     []string
	Skipped     []string
	Overwritten []string
}

func newInitCmd() *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create default config and snippet files",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := runInit(opts)
			return err
		},
	}

	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite existing config and snippet files")
	cmd.Flags().BoolVar(&opts.Missing, "missing", false, "restore missing default snippet files")

	return cmd
}

func runInit(opts initOptions) (initResult, error) {
	configPath := cfgFile
	if configPath == "" {
		path, err := defaultConfigPath()
		if err != nil {
			return initResult{}, err
		}
		configPath = path
	}

	return writeInitialConfig(configPath, defaults.SnippetFiles(), opts)
}

func writeInitialConfig(configPath string, snippetFiles map[string][]byte, opts initOptions) (initResult, error) {
	var result initResult

	configData, err := defaultConfigYAML()
	if err != nil {
		return result, err
	}
	if err := writeTrackedFile(configPath, configData, opts, &result); err != nil {
		return result, err
	}

	names := make([]string, 0, len(snippetFiles))
	for name := range snippetFiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(filepath.Dir(configPath), "snippets", name)
		if err := writeTrackedFile(path, snippetFiles[name], opts, &result); err != nil {
			return result, err
		}
	}

	return result, nil
}

func writeTrackedFile(path string, data []byte, opts initOptions, result *initResult) error {
	_, err := os.Stat(path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if exists && opts.Force {
		if err := writeFileIfAllowed(path, data, true); err != nil {
			return err
		}
		result.Overwritten = append(result.Overwritten, path)
		return nil
	}

	if exists && opts.Missing {
		// Missing mode restores absent files only; existing files match default skip behavior.
		result.Skipped = append(result.Skipped, path)
		return nil
	}

	if exists {
		result.Skipped = append(result.Skipped, path)
		return nil
	}

	if err := writeFileIfAllowed(path, data, false); err != nil {
		return err
	}
	result.Created = append(result.Created, path)
	return nil
}

func writeFileIfAllowed(path string, data []byte, overwrite bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if overwrite {
		return os.WriteFile(path, data, 0644)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
