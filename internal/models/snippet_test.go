package models

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// loadTestConfig loads the test configuration from testdata
func loadTestConfig(t *testing.T) *Config {
	t.Helper()

	// Load main config
	configPath := filepath.Join("..", "..", "testdata", "config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read test config: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse test config: %v", err)
	}

	// Load transform templates
	templatesPath := filepath.Join("..", "..", "testdata", "transform_templates.yaml")
	templatesData, err := os.ReadFile(templatesPath)
	if err != nil {
		t.Fatalf("Failed to read transform templates: %v", err)
	}

	var templatesConfig Config
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

	var typesConfig Config
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

	var snippetsConfig Config
	if err := yaml.Unmarshal(snippetsData, &snippetsConfig); err != nil {
		t.Fatalf("Failed to parse test snippets: %v", err)
	}
	config.Snippets = snippetsConfig.Snippets

	return &config
}

// TestProcessTemplate_NoVariables tests snippets with no variables
func TestProcessTemplate_NoVariables(t *testing.T) {
	config := loadTestConfig(t)
	snippet := config.Snippets["simple-no-vars"]

	result, err := snippet.ProcessTemplate(nil, config)
	if err != nil {
		t.Fatalf("ProcessTemplate failed: %v", err)
	}

	expected := "echo Hello World"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestProcessTemplate_SimpleVariables tests basic variable substitution
func TestProcessTemplate_SimpleVariables(t *testing.T) {
	config := loadTestConfig(t)
	snippet := config.Snippets["simple-with-vars"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "all values provided",
			values:   map[string]string{"message": "Hi", "name": "Alice"},
			expected: "echo Hi Alice",
		},
		{
			name:     "use default for name",
			values:   map[string]string{"message": "Hello"},
			expected: "echo Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_DefaultValues tests default value handling
func TestProcessTemplate_DefaultValues(t *testing.T) {
	config := loadTestConfig(t)
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
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_BooleanTransform tests boolean transformations
func TestProcessTemplate_BooleanTransform(t *testing.T) {
	config := loadTestConfig(t)
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
			name:     "debug true",
			values:   map[string]string{"verbose": "false", "debug": "true"},
			expected: "app  -d",
		},
		{
			name:     "both true",
			values:   map[string]string{"verbose": "true", "debug": "true"},
			expected: "app --verbose -d",
		},
		{
			name:     "yes as true",
			values:   map[string]string{"verbose": "yes", "debug": "false"},
			expected: "app --verbose ",
		},
		{
			name:     "1 as true",
			values:   map[string]string{"verbose": "1", "debug": "false"},
			expected: "app --verbose ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_TransformTemplate tests using transform templates
func TestProcessTemplate_TransformTemplate(t *testing.T) {
	config := loadTestConfig(t)
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
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_ValuePattern tests value pattern transformations
func TestProcessTemplate_ValuePattern(t *testing.T) {
	config := loadTestConfig(t)
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
		{
			name:     "yaml format",
			values:   map[string]string{"output_format": "yaml"},
			expected: "app --format=yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_EmptyValueTransform tests empty value transformations
func TestProcessTemplate_EmptyValueTransform(t *testing.T) {
	config := loadTestConfig(t)
	snippet := config.Snippets["snippet-with-empty-value"]

	tests := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "empty flag",
			values:   map[string]string{"optional_flag": ""},
			expected: "app ",
		},
		{
			name:     "flag with value",
			values:   map[string]string{"optional_flag": "test"},
			expected: "app --flag=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_ComputedSimple tests simple computed variables
func TestProcessTemplate_ComputedSimple(t *testing.T) {
	config := loadTestConfig(t)
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
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_ComputedConditional tests conditional computed variables
func TestProcessTemplate_ComputedConditional(t *testing.T) {
	config := loadTestConfig(t)
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
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_MultipleTransforms tests snippets with multiple transform types
func TestProcessTemplate_MultipleTransforms(t *testing.T) {
	config := loadTestConfig(t)
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
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestProcessTemplate_ComplexComputed tests complex computed variables
func TestProcessTemplate_ComplexComputed(t *testing.T) {
	config := loadTestConfig(t)
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
			result, err := snippet.ProcessTemplate(tt.values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestValidate_Required tests required field validation
func TestValidate_Required(t *testing.T) {
	variable := Variable{
		Name:     "test",
		Required: true,
	}

	err := variable.Validate("")
	if err == nil {
		t.Error("Expected error for empty required field")
	}

	err = variable.Validate("value")
	if err != nil {
		t.Errorf("Unexpected error for non-empty required field: %v", err)
	}
}

// TestValidate_Enum tests enum validation
func TestValidate_Enum(t *testing.T) {
	variable := Variable{
		Name: "test",
		Validation: &Validation{
			Enum: []string{"dev", "staging", "prod"},
		},
	}

	tests := []struct {
		name      string
		value     string
		wantError bool
	}{
		{"valid dev", "dev", false},
		{"valid staging", "staging", false},
		{"valid prod", "prod", false},
		{"invalid value", "test", true},
		{"empty value", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := variable.Validate(tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidate_Range tests range validation
func TestValidate_Range(t *testing.T) {
	variable := Variable{
		Name: "port",
		Validation: &Validation{
			Range: []int{1, 65535},
		},
	}

	tests := []struct {
		name      string
		value     string
		wantError bool
	}{
		{"valid port 80", "80", false},
		{"valid port 8080", "8080", false},
		{"valid port 65535", "65535", false},
		{"valid port 1", "1", false},
		{"invalid port 0", "0", true},
		{"invalid port 65536", "65536", true},
		{"invalid port -1", "-1", true},
		{"invalid not a number", "abc", true},
		{"empty value", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := variable.Validate(tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidate_Pattern tests pattern validation
func TestValidate_Pattern(t *testing.T) {
	variable := Variable{
		Name: "email",
		Validation: &Validation{
			Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
		},
	}

	tests := []struct {
		name      string
		value     string
		wantError bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "user@mail.example.co.uk", false},
		{"invalid no @", "testexample.com", true},
		{"invalid no domain", "test@", true},
		{"invalid no tld", "test@example", true},
		{"empty value", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := variable.Validate(tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidateWithConfig_TypeValidation tests type-based validation
func TestValidateWithConfig_TypeValidation(t *testing.T) {
	config := loadTestConfig(t)

	tests := []struct {
		name      string
		variable  Variable
		value     string
		wantError bool
	}{
		{
			name: "valid port type",
			variable: Variable{
				Name: "port",
				Type: "test_port",
			},
			value:     "8080",
			wantError: false,
		},
		{
			name: "invalid port type - out of range",
			variable: Variable{
				Name: "port",
				Type: "test_port",
			},
			value:     "99999",
			wantError: true,
		},
		{
			name: "valid log level type",
			variable: Variable{
				Name: "log_level",
				Type: "test_log_level",
			},
			value:     "debug",
			wantError: false,
		},
		{
			name: "invalid log level type",
			variable: Variable{
				Name: "log_level",
				Type: "test_log_level",
			},
			value:     "trace",
			wantError: true,
		},
		{
			name: "valid environment type",
			variable: Variable{
				Name: "env",
				Type: "test_environment",
			},
			value:     "prod",
			wantError: false,
		},
		{
			name: "invalid environment type",
			variable: Variable{
				Name: "env",
				Type: "test_environment",
			},
			value:     "local",
			wantError: true,
		},
		{
			name: "valid version pattern type",
			variable: Variable{
				Name: "version",
				Type: "test_version",
			},
			value:     "1.2.3",
			wantError: false,
		},
		{
			name: "valid version pattern type with v prefix",
			variable: Variable{
				Name: "version",
				Type: "test_version",
			},
			value:     "v1.2.3",
			wantError: false,
		},
		{
			name: "invalid version pattern type",
			variable: Variable{
				Name: "version",
				Type: "test_version",
			},
			value:     "1.2",
			wantError: true,
		},
		{
			name: "valid regex type",
			variable: Variable{
				Name: "pattern",
				Type: "regex",
			},
			value:     `^test.*$`,
			wantError: false,
		},
		{
			name: "invalid regex type",
			variable: Variable{
				Name: "pattern",
				Type: "regex",
			},
			value:     `[unclosed`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.variable.ValidateWithConfig(tt.value, config)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateWithConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestProcessTemplate_InvalidTransformTemplate tests error handling for missing templates
func TestProcessTemplate_InvalidTransformTemplate(t *testing.T) {
	config := loadTestConfig(t)

	snippet := Snippet{
		ID:      "test",
		Command: "test <var>",
		Variables: []Variable{
			{
				Name:              "var",
				TransformTemplate: "non-existent-template",
			},
		},
	}

	_, err := snippet.ProcessTemplate(map[string]string{"var": "value"}, config)
	if err == nil {
		t.Error("Expected error for non-existent transform template")
	}
}

// TestProcessTemplate_AllFeaturesCombined tests the comprehensive snippet
func TestProcessTemplate_AllFeaturesCombined(t *testing.T) {
	config := loadTestConfig(t)
	snippet := config.Snippets["snippet-with-all-features"]

	values := map[string]string{
		"environment": "prod",
		"port":        "8080",
		"verbose":     "true",
		"log_level":   "info",
		"extra_flag":  "custom",
	}

	result, err := snippet.ProcessTemplate(values, config)
	if err != nil {
		t.Fatalf("ProcessTemplate failed: %v", err)
	}

	expected := "complex-app --env=prod --port=8080 --verbose --log=info custom"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestProcessTemplate_RegexType tests regex type validation
func TestProcessTemplate_RegexType(t *testing.T) {
	config := loadTestConfig(t)
	snippet := config.Snippets["snippet-with-regex-type"]

	tests := []struct {
		name      string
		pattern   string
		wantError bool
	}{
		{
			name:      "valid regex",
			pattern:   `^test.*$`,
			wantError: false,
		},
		{
			name:      "valid complex regex",
			pattern:   `\d{3}-\d{3}-\d{4}`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[string]string{"pattern": tt.pattern}
			result, err := snippet.ProcessTemplate(values, config)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			expected := "grep " + tt.pattern + " file.txt"
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})
	}
}
