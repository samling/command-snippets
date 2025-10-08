package models

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestConfigLoading tests loading and parsing configuration files
func TestConfigLoading(t *testing.T) {
	testdataPath := filepath.Join("..", "..", "testdata")

	t.Run("load main config", func(t *testing.T) {
		configPath := filepath.Join(testdataPath, "config.yaml")
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatalf("Failed to parse config: %v", err)
		}

		if config.Settings.Interactive.ShowFinalCommand != true {
			t.Error("Expected show_final_command to be true")
		}
		if config.Settings.Interactive.ConfirmBeforeExecute != false {
			t.Error("Expected confirm_before_execute to be false")
		}
		if config.Settings.Selector.Command != "fzf" {
			t.Errorf("Expected selector command 'fzf', got %q", config.Settings.Selector.Command)
		}
	})

	t.Run("load transform templates", func(t *testing.T) {
		templatesPath := filepath.Join(testdataPath, "transform_templates.yaml")
		data, err := os.ReadFile(templatesPath)
		if err != nil {
			t.Fatalf("Failed to read transform templates: %v", err)
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatalf("Failed to parse transform templates: %v", err)
		}

		expectedTemplates := []string{
			"test-namespace",
			"test-port-mapping",
			"test-boolean-flag",
			"test-prefix",
		}

		for _, name := range expectedTemplates {
			if _, exists := config.TransformTemplates[name]; !exists {
				t.Errorf("Expected transform template %q not found", name)
			}
		}

		nsTemplate := config.TransformTemplates["test-namespace"]
		if nsTemplate.Description == "" {
			t.Error("Transform template should have description")
		}
		if nsTemplate.Transform == nil {
			t.Error("Transform template should have transform")
		}
	})

	t.Run("load variable types", func(t *testing.T) {
		typesPath := filepath.Join(testdataPath, "types.yaml")
		data, err := os.ReadFile(typesPath)
		if err != nil {
			t.Fatalf("Failed to read variable types: %v", err)
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatalf("Failed to parse variable types: %v", err)
		}

		expectedTypes := []string{
			"test_port",
			"test_log_level",
			"test_environment",
			"test_email",
			"test_version",
		}

		for _, name := range expectedTypes {
			if _, exists := config.VariableTypes[name]; !exists {
				t.Errorf("Expected variable type %q not found", name)
			}
		}

		portType := config.VariableTypes["test_port"]
		if portType.Default != "8080" {
			t.Errorf("Expected default port '8080', got %q", portType.Default)
		}
		if portType.Validation == nil {
			t.Error("Port type should have validation")
		}
		if len(portType.Validation.Range) != 2 {
			t.Error("Port type should have range validation")
		}

		logType := config.VariableTypes["test_log_level"]
		if logType.Default != "info" {
			t.Errorf("Expected default log level 'info', got %q", logType.Default)
		}
		if len(logType.Validation.Enum) == 0 {
			t.Error("Log level type should have enum validation")
		}
	})

	t.Run("load test snippets", func(t *testing.T) {
		snippetsPath := filepath.Join(testdataPath, "test_snippets.yaml")
		data, err := os.ReadFile(snippetsPath)
		if err != nil {
			t.Fatalf("Failed to read test snippets: %v", err)
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatalf("Failed to parse test snippets: %v", err)
		}

		expectedSnippets := []string{
			"simple-no-vars",
			"simple-with-vars",
			"snippet-with-defaults",
			"snippet-with-enum",
			"snippet-with-range",
			"snippet-with-pattern",
			"snippet-with-boolean",
			"snippet-with-transform-template",
			"snippet-with-value-pattern",
			"snippet-with-computed-simple",
			"snippet-with-computed-conditional",
			"snippet-with-complex-computed",
			"snippet-with-all-features",
		}

		for _, name := range expectedSnippets {
			if _, exists := config.Snippets[name]; !exists {
				t.Errorf("Expected snippet %q not found", name)
			}
		}

		simpleSnippet := config.Snippets["simple-no-vars"]
		if simpleSnippet.ID != "simple-no-vars" {
			t.Errorf("Expected ID 'simple-no-vars', got %q", simpleSnippet.ID)
		}
		if simpleSnippet.Command == "" {
			t.Error("Snippet should have command")
		}

		varSnippet := config.Snippets["simple-with-vars"]
		if len(varSnippet.Variables) != 2 {
			t.Errorf("Expected 2 variables, got %d", len(varSnippet.Variables))
		}
	})
}

// TestTransformTemplateStructure tests transform template structure
func TestTransformTemplateStructure(t *testing.T) {
	config := loadTestConfig(t)

	tests := []struct {
		name          string
		templateName  string
		hasEmptyValue bool
		hasValuePat   bool
		hasTrueValue  bool
		hasFalseValue bool
	}{
		{
			name:          "namespace template",
			templateName:  "test-namespace",
			hasEmptyValue: true,
			hasValuePat:   true,
		},
		{
			name:          "port mapping template",
			templateName:  "test-port-mapping",
			hasEmptyValue: true,
			hasValuePat:   true,
		},
		{
			name:          "boolean flag template",
			templateName:  "test-boolean-flag",
			hasTrueValue:  true,
			hasFalseValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, exists := config.TransformTemplates[tt.templateName]
			if !exists {
				t.Fatalf("Template %q not found", tt.templateName)
			}

			if tmpl.Transform == nil {
				t.Fatal("Transform should not be nil")
			}

			if tt.hasValuePat && tmpl.Transform.ValuePattern == "" {
				t.Error("Expected value_pattern to be set")
			}
			if tt.hasTrueValue && tmpl.Transform.TrueValue == "" {
				t.Error("Expected true_value to be set")
			}
		})
	}
}

// TestVariableTypeStructure tests variable type structure
func TestVariableTypeStructure(t *testing.T) {
	config := loadTestConfig(t)

	tests := []struct {
		name            string
		typeName        string
		hasDefault      bool
		hasRange        bool
		hasEnum         bool
		hasPattern      bool
		expectedDefault string
	}{
		{
			name:            "port type",
			typeName:        "test_port",
			hasDefault:      true,
			hasRange:        true,
			expectedDefault: "8080",
		},
		{
			name:            "log level type",
			typeName:        "test_log_level",
			hasDefault:      true,
			hasEnum:         true,
			expectedDefault: "info",
		},
		{
			name:       "environment type",
			typeName:   "test_environment",
			hasDefault: false,
			hasEnum:    true,
		},
		{
			name:       "email type",
			typeName:   "test_email",
			hasDefault: false,
			hasPattern: true,
		},
		{
			name:            "version type",
			typeName:        "test_version",
			hasDefault:      true,
			hasPattern:      true,
			expectedDefault: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varType, exists := config.VariableTypes[tt.typeName]
			if !exists {
				t.Fatalf("Variable type %q not found", tt.typeName)
			}

			if tt.hasDefault {
				if varType.Default == "" {
					t.Error("Expected default to be set")
				}
				if tt.expectedDefault != "" && varType.Default != tt.expectedDefault {
					t.Errorf("Expected default %q, got %q", tt.expectedDefault, varType.Default)
				}
			}

			if varType.Validation == nil && (tt.hasRange || tt.hasEnum || tt.hasPattern) {
				t.Fatal("Expected validation to be set")
			}

			if tt.hasRange {
				if len(varType.Validation.Range) != 2 {
					t.Error("Expected range validation with 2 values")
				}
			}

			if tt.hasEnum {
				if len(varType.Validation.Enum) == 0 {
					t.Error("Expected enum validation with values")
				}
			}

			if tt.hasPattern {
				if varType.Validation.Pattern == "" {
					t.Error("Expected pattern validation to be set")
				}
			}
		})
	}
}

// TestSnippetStructure tests snippet structure
func TestSnippetStructure(t *testing.T) {
	config := loadTestConfig(t)

	tests := []struct {
		name               string
		snippetID          string
		expectedVarCount   int
		hasComputedVars    bool
		hasTransformTmpl   bool
		hasInlineTransform bool
	}{
		{
			name:             "simple no vars",
			snippetID:        "simple-no-vars",
			expectedVarCount: 0,
		},
		{
			name:             "simple with vars",
			snippetID:        "simple-with-vars",
			expectedVarCount: 2,
		},
		{
			name:             "with transform template",
			snippetID:        "snippet-with-transform-template",
			expectedVarCount: 1,
			hasTransformTmpl: true,
		},
		{
			name:               "with boolean",
			snippetID:          "snippet-with-boolean",
			expectedVarCount:   2,
			hasInlineTransform: true,
		},
		{
			name:             "with computed simple",
			snippetID:        "snippet-with-computed-simple",
			expectedVarCount: 3,
			hasComputedVars:  true,
		},
		{
			name:             "with complex computed",
			snippetID:        "snippet-with-complex-computed",
			expectedVarCount: 5,
			hasComputedVars:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snippet, exists := config.Snippets[tt.snippetID]
			if !exists {
				t.Fatalf("Snippet %q not found", tt.snippetID)
			}

			if snippet.ID != tt.snippetID {
				t.Errorf("Expected ID %q, got %q", tt.snippetID, snippet.ID)
			}

			if snippet.Command == "" {
				t.Error("Snippet should have command")
			}

			if len(snippet.Variables) != tt.expectedVarCount {
				t.Errorf("Expected %d variables, got %d", tt.expectedVarCount, len(snippet.Variables))
			}

			if tt.hasComputedVars {
				hasComputed := false
				for _, v := range snippet.Variables {
					if v.Computed {
						hasComputed = true
						break
					}
				}
				if !hasComputed {
					t.Error("Expected to have computed variables")
				}
			}

			if tt.hasTransformTmpl {
				hasTemplate := false
				for _, v := range snippet.Variables {
					if v.TransformTemplate != "" {
						hasTemplate = true
						break
					}
				}
				if !hasTemplate {
					t.Error("Expected to have transform template")
				}
			}

			if tt.hasInlineTransform {
				hasTransform := false
				for _, v := range snippet.Variables {
					if v.Transform != nil {
						hasTransform = true
						break
					}
				}
				if !hasTransform {
					t.Error("Expected to have inline transform")
				}
			}
		})
	}
}
