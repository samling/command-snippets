package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/samling/command-snippets/internal/models"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	cfgFile        string
	config         *models.Config
	generateConfig bool
)

// version is overridden at link time via -X. "dev" is the default for
// `go build` / `go install` invocations without ldflags.
var version = "dev"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "cs",
	Version: version,
	Short:   "Command Snippets - Advanced command template toolkit with intelligent variable substitution",
	Long: `CS (Command Snippets) is a powerful CLI tool for managing command templates with advanced variable substitution.

Features:
- Intelligent template-based variable transformation
- Conditional logic and smart defaults
- Interactive template execution
- Reusable transformation patterns
- Tag-based organization
- Complex variable composition`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !commandNeedsConfig(cmd) {
			return nil
		}
		return initConfig()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if generateConfig {
			data, err := defaultConfigYAML()
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}
			fmt.Print("# CS (Command Snippets) Configuration\n# Generated default configuration\n\n")
			fmt.Print(string(data))
			return nil
		}
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.EnableTraverseRunHooks = true

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/cs/config.yaml, fallback $HOME/.config/cs/config.yaml)")
	rootCmd.Flags().BoolVar(&generateConfig, "generate-config", false, "generate default config to stdout")

	// Add subcommands
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newAddCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newExecCmd())
	rootCmd.AddCommand(newEditCmd())
	rootCmd.AddCommand(newDescribeCmd())
	rootCmd.AddCommand(newShowCmd())
}

// initConfig reads in config file and ENV variables.
func initConfig() error {
	if cfgFile == "" {
		path, err := defaultConfigPath()
		if err != nil {
			return err
		}
		cfgFile = path
	}

	// Load configuration
	var err error
	config, err = loadConfig(cfgFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			config = createDefaultConfig()
			return missingConfigError(cfgFile)
		}
		return fmt.Errorf("error loading config: %w", err)
	}
	return nil
}

func commandNeedsConfig(cmd *cobra.Command) bool {
	return cmd.Parent() != nil && cmd.CommandPath() != "cs init"
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
	return fmt.Errorf("config file not found at %s; run `cs init` to create one", path)
}

// loadConfig loads configuration from YAML file and merges additional snippet files
func loadConfig(filename string) (*models.Config, error) {
	// Load main config file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Initialize snippets map if nil
	if cfg.Snippets == nil {
		cfg.Snippets = make(map[string]models.Snippet)
	}

	// Mark all snippets from main config as global
	for name, snippet := range cfg.Snippets {
		snippet.Source = models.SourceGlobal
		cfg.Snippets[name] = snippet
	}

	// Load additional configuration files if specified
	if err := loadAdditionalConfigs(&cfg, filename); err != nil {
		return nil, fmt.Errorf("loading additional configs: %w", err)
	}

	// Load local project snippets if .csnippets file exists in current directory
	if err := loadLocalSnippets(&cfg); err != nil {
		return nil, fmt.Errorf("loading local snippets: %w", err)
	}

	return &cfg, nil
}

// loadAdditionalConfigs loads and merges additional configuration files.
// Files are read and parsed in parallel; merging stays serial so the
// "overwrite" warnings remain in deterministic order.
func loadAdditionalConfigs(cfg *models.Config, configDir string) error {
	baseDir := filepath.Dir(configDir)

	var paths []string
	for _, additionalPath := range cfg.Settings.AdditionalConfigs {
		configPath := expandPath(additionalPath)
		if !filepath.IsAbs(configPath) {
			configPath = filepath.Join(baseDir, configPath)
		}

		matches, err := filepath.Glob(configPath)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %s: %w", configPath, err)
		}
		if len(matches) == 0 {
			paths = append(paths, configPath)
		} else {
			paths = append(paths, matches...)
		}
	}

	type loaded struct {
		path string
		cfg  models.Config
		err  error
	}
	results := make([]loaded, len(paths))
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			results[i].path = p
			results[i].cfg, results[i].err = readConfigFile(p)
		}(i, p)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			if os.IsNotExist(r.err) {
				fmt.Printf("Warning: Additional config file not found: %s\n", r.path)
				continue
			}
			return fmt.Errorf("loading additional config file %s: %w", r.path, r.err)
		}
		mergeConfig(cfg, &r.cfg, r.path, models.SourceGlobal)
	}
	return nil
}

// readConfigFile reads and parses a YAML config file without merging.
func readConfigFile(filename string) (models.Config, error) {
	var c models.Config
	data, err := os.ReadFile(filename)
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return c, nil
}

// loadConfigFileWithSource reads, parses, and merges a config file in one step.
// Used for the local .csnippets path where parallelism doesn't apply.
func loadConfigFileWithSource(cfg *models.Config, filename string, source models.SnippetSource) error {
	additionalConfig, err := readConfigFile(filename)
	if err != nil {
		return err
	}
	mergeConfig(cfg, &additionalConfig, filename, source)
	return nil
}

// mergeConfig merges src into dst. Snippets gain the given source label.
func mergeConfig(dst, src *models.Config, filename string, source models.SnippetSource) {
	if dst.TransformTemplates == nil {
		dst.TransformTemplates = make(map[string]models.TransformTemplate)
	}
	if dst.VariableTypes == nil {
		dst.VariableTypes = make(map[string]models.VariableType)
	}
	if dst.Snippets == nil {
		dst.Snippets = make(map[string]models.Snippet)
	}

	for name, template := range src.TransformTemplates {
		if _, exists := dst.TransformTemplates[name]; exists {
			fmt.Printf("Warning: Transform template '%s' from %s overwrites existing template\n", name, filename)
		}
		dst.TransformTemplates[name] = template
	}
	for name, varType := range src.VariableTypes {
		if _, exists := dst.VariableTypes[name]; exists {
			fmt.Printf("Warning: Variable type '%s' from %s overwrites existing type\n", name, filename)
		}
		dst.VariableTypes[name] = varType
	}
	for name, snippet := range src.Snippets {
		if _, exists := dst.Snippets[name]; exists {
			fmt.Printf("Warning: Snippet '%s' from %s overwrites existing snippet\n", name, filename)
		}
		snippet.Source = source
		dst.Snippets[name] = snippet
	}
}

// loadLocalSnippets loads snippets from a local .csnippets file in the current directory
func loadLocalSnippets(cfg *models.Config) error {
	// Check if .csnippets file exists in current working directory
	localSnippetsFile := ".csnippets"
	if _, err := os.Stat(localSnippetsFile); os.IsNotExist(err) {
		// No local snippets file, that's fine
		return nil
	}

	// Load the local snippets file with local source marking
	if err := loadConfigFileWithSource(cfg, localSnippetsFile, models.SourceLocal); err != nil {
		return fmt.Errorf("loading local snippets from %s: %w", localSnippetsFile, err)
	}

	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// saveConfig saves configuration to YAML file
func saveConfig(cfg *models.Config, filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func defaultConfigYAML() ([]byte, error) {
	return yaml.Marshal(createDefaultConfig())
}

// createDefaultConfig creates a minimal stub configuration
func createDefaultConfig() *models.Config {
	return &models.Config{
		TransformTemplates: make(map[string]models.TransformTemplate),
		VariableTypes:      make(map[string]models.VariableType),
		Snippets:           make(map[string]models.Snippet),
		Settings: models.Settings{
			AdditionalConfigs: []string{
				"snippets/*.yaml",
			},
			Selector: models.SelectorConfig{
				Command: "fzf",
				Options: "--height 40% --reverse --border --sort",
			},
		},
	}
}
