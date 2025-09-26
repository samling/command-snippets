package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samling/command-snippets/internal/models"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	cfgFile        string
	config         *models.Config
	generateConfig bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cs",
	Short: "Command Snippets - Advanced command template toolkit with intelligent variable substitution",
	Long: `CS (Command Snippets) is a powerful CLI tool for managing command templates with advanced variable substitution.

Features:
- Intelligent template-based variable transformation
- Conditional logic and smart defaults  
- Interactive template execution
- Reusable transformation patterns
- Tag-based organization
- Complex variable composition`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if generateConfig {
			defaultConfig := createDefaultConfig()
			data, err := yaml.Marshal(defaultConfig)
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
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/cs/config.yaml)")
	rootCmd.Flags().BoolVar(&generateConfig, "generate-config", false, "generate default config to stdout")

	// Add subcommands
	rootCmd.AddCommand(newAddCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newExecCmd())
	rootCmd.AddCommand(newEditCmd())
	rootCmd.AddCommand(newDescribeCmd())
	rootCmd.AddCommand(newShowCmd())
}

// initConfig reads in config file and ENV variables.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name "config"
		cfgFile = filepath.Join(home, ".config", "cs", "config.yaml")
	}

	// Load configuration
	var err error
	config, err = loadConfig(cfgFile)
	if err != nil {
		// Create default config if file doesn't exist
		if os.IsNotExist(err) {
			config = createDefaultConfig()
			if err := saveConfig(config, cfgFile); err != nil {
				fmt.Printf("Warning: Could not save default config: %v\n", err)
			}
		} else {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
	}
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

// loadAdditionalConfigs loads and merges additional configuration files
func loadAdditionalConfigs(cfg *models.Config, configDir string) error {
	baseDir := filepath.Dir(configDir)

	// Load additional configuration files
	for _, additionalPath := range cfg.Settings.AdditionalConfigs {
		configPath := expandPath(additionalPath)
		if !filepath.IsAbs(configPath) {
			configPath = filepath.Join(baseDir, configPath)
		}

		// Expand glob patterns
		matches, err := filepath.Glob(configPath)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %s: %w", configPath, err)
		}

		if len(matches) == 0 {
			// If no matches found, treat as a literal path and check if it exists
			if err := loadConfigFile(cfg, configPath); err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("Warning: Additional config file not found: %s\n", configPath)
					continue
				}
				return fmt.Errorf("loading additional config file %s: %w", configPath, err)
			}
		} else {
			// Process all matched files
			for _, matchedFile := range matches {
				if err := loadConfigFile(cfg, matchedFile); err != nil {
					if os.IsNotExist(err) {
						fmt.Printf("Warning: Additional config file not found: %s\n", matchedFile)
						continue
					}
					return fmt.Errorf("loading additional config file %s: %w", matchedFile, err)
				}
			}
		}
	}

	return nil
}

// loadConfigFile loads a config file and merges it into the main config
func loadConfigFile(cfg *models.Config, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var additionalConfig models.Config
	if err := yaml.Unmarshal(data, &additionalConfig); err != nil {
		return err
	}

	// Initialize maps if they don't exist in the main config
	if cfg.TransformTemplates == nil {
		cfg.TransformTemplates = make(map[string]models.TransformTemplate)
	}
	if cfg.VariableTypes == nil {
		cfg.VariableTypes = make(map[string]models.VariableType)
	}
	if cfg.Snippets == nil {
		cfg.Snippets = make(map[string]models.Snippet)
	}

	// Merge transform templates
	for name, template := range additionalConfig.TransformTemplates {
		if _, exists := cfg.TransformTemplates[name]; exists {
			fmt.Printf("Warning: Transform template '%s' from %s overwrites existing template\n", name, filename)
		}
		cfg.TransformTemplates[name] = template
	}

	// Merge variable types
	for name, varType := range additionalConfig.VariableTypes {
		if _, exists := cfg.VariableTypes[name]; exists {
			fmt.Printf("Warning: Variable type '%s' from %s overwrites existing type\n", name, filename)
		}
		cfg.VariableTypes[name] = varType
	}

	// Merge snippets
	for name, snippet := range additionalConfig.Snippets {
		if _, exists := cfg.Snippets[name]; exists {
			fmt.Printf("Warning: Snippet '%s' from %s overwrites existing snippet\n", name, filename)
		}
		cfg.Snippets[name] = snippet
	}

	return nil
}

// loadLocalSnippets loads snippets from a local .csnippets file in the current directory
func loadLocalSnippets(cfg *models.Config) error {
	// Check if .csnippets file exists in current working directory
	localSnippetsFile := ".csnippets"
	if _, err := os.Stat(localSnippetsFile); os.IsNotExist(err) {
		// No local snippets file, that's fine
		return nil
	}

	// Load the local snippets file
	if err := loadConfigFile(cfg, localSnippetsFile); err != nil {
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
			Interactive: models.InteractiveConfig{
				ConfirmBeforeExecute: false,
				ShowFinalCommand:     true,
			},
			Selector: models.SelectorConfig{
				Command: "fzf",
				Options: "--height 40% --reverse --border --sort",
			},
		},
	}
}
