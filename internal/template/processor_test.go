package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/samling/command-snippets/internal/models"
	"gopkg.in/yaml.v3"
)

// loadTestConfig loads the test configuration from testdata
func loadTestConfig(t *testing.T) *models.Config {
	t.Helper()

	// Load main config
	configPath := filepath.Join("..", "..", "testdata", "config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read test config: %v", err)
	}

	var config models.Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse test config: %v", err)
	}

	// Load transform templates
	templatesPath := filepath.Join("..", "..", "testdata", "transform_templates.yaml")
	templatesData, err := os.ReadFile(templatesPath)
	if err != nil {
		t.Fatalf("Failed to read transform templates: %v", err)
	}

	var templatesConfig models.Config
	if err := yaml.Unmarshal(templatesData, &templatesConfig); err != nil {
		t.Fatalf("Failed to parse transform templates: %v", err)
	}
	config.TransformTemplates = templatesConfig.TransformTemplates

	// Load variable types
	typesPath := filepath.Join("..", "..", "testdata", "types.yaml")
	typesData, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("Failed to read variable types: %v", err)
	}

	var typesConfig models.Config
	if err := yaml.Unmarshal(typesData, &typesConfig); err != nil {
		t.Fatalf("Failed to parse variable types: %v", err)
	}
	config.VariableTypes = typesConfig.VariableTypes

	// Load test snippets
	snippetsPath := filepath.Join("..", "..", "testdata", "test_snippets.yaml")
	snippetsData, err := os.ReadFile(snippetsPath)
	if err != nil {
		t.Fatalf("Failed to read test snippets: %v", err)
	}

	var snippetsConfig models.Config
	if err := yaml.Unmarshal(snippetsData, &snippetsConfig); err != nil {
		t.Fatalf("Failed to parse test snippets: %v", err)
	}
	config.Snippets = snippetsConfig.Snippets

	return &config
}

// TestNewProcessor tests processor creation
func TestNewProcessor(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)

	if processor == nil {
		t.Fatal("NewProcessor returned nil")
	}

	if processor.config != config {
		t.Error("Processor config not set correctly")
	}
}

// TestProcessSnippet_NoVariables tests processing snippets without variables
func TestProcessSnippet_NoVariables(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["simple-no-vars"]

	result, err := processor.ProcessSnippet(&snippet, nil)
	if err != nil {
		t.Fatalf("ProcessSnippet failed: %v", err)
	}

	expected := "echo Hello World"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestProcessSnippet_SimpleVariables tests basic variable substitution
func TestProcessSnippet_SimpleVariables(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["simple-with-vars"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "all values provided",
			values:   map[string]string{"message": "Hello", "name": "World"},
			expected: "echo Hello World",
		},
		{
			name:     "use default for name",
			values:   map[string]string{"message": "Hello"},
			expected: "echo Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_WithDefaults tests default value handling
func TestProcessSnippet_WithDefaults(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-defaults"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "with custom timeout",
			values:   map[string]string{"url": "http://example.com", "timeout": "60"},
			expected: "curl http://example.com 60",
		},
		{
			name:     "with default timeout",
			values:   map[string]string{"url": "http://example.com"},
			expected: "curl http://example.com 30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_BooleanTransform tests boolean transformations
func TestProcessSnippet_BooleanTransform(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-boolean"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "both false",
			values:   map[string]string{"verbose": "false", "debug": "false"},
			expected: "app  ",
		},
		{
			name:     "verbose true",
			values:   map[string]string{"verbose": "true", "debug": "false"},
			expected: "app --verbose ",
		},
		{
			name:     "both true",
			values:   map[string]string{"verbose": "true", "debug": "true"},
			expected: "app --verbose -d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_TransformTemplate tests using transform templates
func TestProcessSnippet_TransformTemplate(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-transform-template"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "empty namespace",
			values:   map[string]string{"namespace": ""},
			expected: "kubectl get pods ",
		},
		{
			name:     "all namespaces",
			values:   map[string]string{"namespace": "all"},
			expected: "kubectl get pods -A",
		},
		{
			name:     "specific namespace",
			values:   map[string]string{"namespace": "default"},
			expected: "kubectl get pods -n default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_ValuePattern tests value pattern transformations
func TestProcessSnippet_ValuePattern(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-value-pattern"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "empty value",
			values:   map[string]string{"output_format": ""},
			expected: "app ",
		},
		{
			name:     "json format",
			values:   map[string]string{"output_format": "json"},
			expected: "app --format=json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_ComputedSimple tests simple computed variables
func TestProcessSnippet_ComputedSimple(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-computed-simple"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name: "pod resource",
			values: map[string]string{
				"resource_type": "pod",
				"resource_name": "my-pod",
			},
			expected: "app pod/my-pod",
		},
		{
			name: "service resource",
			values: map[string]string{
				"resource_type": "service",
				"resource_name": "my-service",
			},
			expected: "app service/my-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_ComputedConditional tests conditional computed variables
func TestProcessSnippet_ComputedConditional(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-computed-conditional"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name: "same port",
			values: map[string]string{
				"host_port":   "8080",
				"target_port": "",
			},
			expected: "server 8080:8080",
		},
		{
			name: "different ports",
			values: map[string]string{
				"host_port":   "8080",
				"target_port": "80",
			},
			expected: "server 8080:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_ComplexComputed tests complex computed variables
func TestProcessSnippet_ComplexComputed(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-complex-computed"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name: "minimal",
			values: map[string]string{
				"image_name": "nginx",
				"port":       "",
				"volume":     "",
				"detach":     "false",
			},
			expected: "docker run  nginx",
		},
		{
			name: "with port",
			values: map[string]string{
				"image_name": "nginx",
				"port":       "8080:80",
				"volume":     "",
				"detach":     "false",
			},
			expected: "docker run -p 8080:80  nginx",
		},
		{
			name: "detached with volume",
			values: map[string]string{
				"image_name": "nginx",
				"port":       "",
				"volume":     "/data:/app",
				"detach":     "true",
			},
			expected: "docker run -d -v /data:/app  nginx",
		},
		{
			name: "all options",
			values: map[string]string{
				"image_name": "nginx",
				"port":       "8080:80",
				"volume":     "/data:/app",
				"detach":     "true",
			},
			expected: "docker run -d -p 8080:80 -v /data:/app  nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_MultipleTransforms tests snippets with multiple transform types
func TestProcessSnippet_MultipleTransforms(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-multiple-transforms"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name: "all defaults",
			values: map[string]string{
				"namespace":   "",
				"output":      "",
				"show_labels": "false",
			},
			expected: "kubectl get pods   ",
		},
		{
			name: "all namespaces, json output, show labels",
			values: map[string]string{
				"namespace":   "all",
				"output":      "json",
				"show_labels": "true",
			},
			expected: "kubectl get pods -A -o json --show-labels",
		},
		{
			name: "specific namespace, wide output",
			values: map[string]string{
				"namespace":   "kube-system",
				"output":      "wide",
				"show_labels": "false",
			},
			expected: "kubectl get pods -n kube-system -o wide ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_AllFeaturesCombined tests the comprehensive snippet
func TestProcessSnippet_AllFeaturesCombined(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-all-features"]

	values := map[string]string{
		"environment": "prod",
		"port":        "8080",
		"verbose":     "true",
		"log_level":   "info",
		"extra_flag":  "custom",
	}

	result, err := processor.ProcessSnippet(&snippet, values)
	if err != nil {
		t.Fatalf("ProcessSnippet failed: %v", err)
	}

	expected := "complex-app --env=prod --port=8080 --verbose --log=info custom"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestProcessSnippet_ErrorHandling tests error scenarios
func TestProcessSnippet_ErrorHandling(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)

	tests := []struct {
		name    string
		snippet models.Snippet
		values  map[string]string
		wantErr bool
	}{
		{
			name: "invalid transform template",
			snippet: models.Snippet{
				ID:      "test",
				Command: "test <var>",
				Variables: []models.Variable{
					{
						Name:              "var",
						TransformTemplate: "non-existent-template",
					},
				},
			},
			values:  map[string]string{"var": "value"},
			wantErr: true,
		},
		{
			name: "invalid compose template syntax",
			snippet: models.Snippet{
				ID:      "test",
				Command: "test <var>",
				Variables: []models.Variable{
					{
						Name:     "var",
						Computed: true,
						Transform: &models.Transform{
							Compose: "{{.invalid syntax",
						},
					},
				},
			},
			values:  map[string]string{},
			wantErr: true,
		},
		{
			name: "invalid value pattern syntax",
			snippet: models.Snippet{
				ID:      "test",
				Command: "test <var>",
				Variables: []models.Variable{
					{
						Name: "var",
						Transform: &models.Transform{
							ValuePattern: "{{.invalid syntax",
						},
					},
				},
			},
			values:  map[string]string{"var": "value"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processor.ProcessSnippet(&tt.snippet, tt.values)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessSnippet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestProcessSnippet_WithEnum tests enum validation in processing
func TestProcessSnippet_WithEnum(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-enum"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "valid log level debug",
			values:   map[string]string{"log_level": "debug"},
			expected: "app --log-level debug",
		},
		{
			name:     "valid log level info",
			values:   map[string]string{"log_level": "info"},
			expected: "app --log-level info",
		},
		{
			name:     "use default",
			values:   map[string]string{},
			expected: "app --log-level info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_WithRange tests range validation in processing
func TestProcessSnippet_WithRange(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-range"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "valid port",
			values:   map[string]string{"port": "3000"},
			expected: "server --port 3000",
		},
		{
			name:     "use default port",
			values:   map[string]string{},
			expected: "server --port 8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_WithPattern tests pattern validation in processing
func TestProcessSnippet_WithPattern(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-pattern"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "valid version",
			values:   map[string]string{"version": "1.2.3"},
			expected: "deploy --version 1.2.3",
		},
		{
			name:     "valid version with v",
			values:   map[string]string{"version": "v2.0.0"},
			expected: "deploy --version v2.0.0",
		},
		{
			name:     "use default version",
			values:   map[string]string{},
			expected: "deploy --version 1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessSnippet_RegexType tests regex type validation
func TestProcessSnippet_RegexType(t *testing.T) {
	config := loadTestConfig(t)
	processor := NewProcessor(config)
	snippet := config.Snippets["snippet-with-regex-type"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "simple regex",
			values:   map[string]string{"pattern": "^test.*$"},
			expected: "grep ^test.*$ file.txt",
		},
		{
			name:     "complex regex",
			values:   map[string]string{"pattern": `\d{3}-\d{3}-\d{4}`},
			expected: `grep \d{3}-\d{3}-\d{4} file.txt`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, tt.values)
			if err != nil {
				t.Fatalf("ProcessSnippet failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
