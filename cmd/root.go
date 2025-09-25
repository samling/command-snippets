package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"tplkit/internal/models"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	cfgFile string
	config  *models.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tplkit",
	Short: "Advanced command template toolkit with intelligent variable substitution",
	Long: `TplKit is a powerful CLI tool for managing command templates with advanced variable substitution.

Features:
- Intelligent template-based variable transformation
- Conditional logic and smart defaults  
- Interactive template execution
- Reusable transformation patterns
- Tag-based organization
- Complex variable composition`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/tplkit/tplkit.yaml)")

	// Add subcommands
	rootCmd.AddCommand(newAddCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newExecCmd())
	rootCmd.AddCommand(newEditCmd())
}

// initConfig reads in config file and ENV variables.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name "tplkit" (without extension)
		cfgFile = filepath.Join(home, ".config", "tplkit", "tplkit.yaml")
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

// loadConfig loads configuration from YAML file
func loadConfig(filename string) (*models.Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
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

// createDefaultConfig creates a minimal default configuration
func createDefaultConfig() *models.Config {
	home, _ := os.UserHomeDir()
	return &models.Config{
		TransformTemplates: make(map[string]models.TransformTemplate),
		VariableTypes:      make(map[string]models.VariableType),
		Snippets:           make(map[string]models.Snippet),
		Settings: models.Settings{
			SnippetFile: filepath.Join(home, ".config", "tplkit", "snippets.yaml"),
			Interactive: models.InteractiveConfig{
				ConfirmBeforeExecute: true,
				ShowFinalCommand:     true,
			},
			Selector: models.SelectorConfig{
				Command: "fzf",
				Options: "--height 40% --reverse --border",
			},
		},
	}
}
