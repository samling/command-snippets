package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/template"
	"gopkg.in/yaml.v3"
)

// loadTestConfig loads the complete test configuration
func loadTestConfig(t *testing.T) *models.Config {
	t.Helper()

	testdataPath := filepath.Join("..", "testdata")

	// Load main config
	configPath := filepath.Join(testdataPath, "config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read test config: %v", err)
	}

	var config models.Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse test config: %v", err)
	}

	// Load transform templates
	templatesPath := filepath.Join(testdataPath, "transform_templates.yaml")
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
	typesPath := filepath.Join(testdataPath, "types.yaml")
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
	snippetsPath := filepath.Join(testdataPath, "test_snippets.yaml")
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

// TestEndToEnd_SimpleSnippet tests a complete workflow with a simple snippet
func TestEndToEnd_SimpleSnippet(t *testing.T) {
	config := loadTestConfig(t)
	processor := template.NewProcessor(config)
	snippet := config.Snippets["simple-with-vars"]

	values := map[string]string{
		"message": "Hello",
		"name":    "World",
	}

	result, err := processor.ProcessSnippet(&snippet, values)
	if err != nil {
		t.Fatalf("Failed to process snippet: %v", err)
	}

	expected := "echo Hello World"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestEndToEnd_KubernetesWorkflow tests a Kubernetes-style workflow
func TestEndToEnd_KubernetesWorkflow(t *testing.T) {
	config := loadTestConfig(t)
	processor := template.NewProcessor(config)

	scenarios := []struct {
		name     string
		snippet  string
		values   map[string]string
		expected string
	}{
		{
			name:    "get pods in all namespaces",
			snippet: "snippet-with-transform-template",
			values: map[string]string{
				"namespace": "all",
			},
			expected: "kubectl get pods -A",
		},
		{
			name:    "get pods in specific namespace",
			snippet: "snippet-with-transform-template",
			values: map[string]string{
				"namespace": "kube-system",
			},
			expected: "kubectl get pods -n kube-system",
		},
		{
			name:    "get pods with output format",
			snippet: "snippet-with-multiple-transforms",
			values: map[string]string{
				"namespace":   "default",
				"output":      "json",
				"show_labels": "true",
			},
			expected: "kubectl get pods -n default -o json --show-labels",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			snippet := config.Snippets[scenario.snippet]
			result, err := processor.ProcessSnippet(&snippet, scenario.values)
			if err != nil {
				t.Fatalf("Failed to process snippet: %v", err)
			}
			if result != scenario.expected {
				t.Errorf("Expected %q, got %q", scenario.expected, result)
			}
		})
	}
}

// TestEndToEnd_DockerWorkflow tests a Docker-style workflow
func TestEndToEnd_DockerWorkflow(t *testing.T) {
	config := loadTestConfig(t)
	processor := template.NewProcessor(config)

	scenarios := []struct {
		name     string
		snippet  string
		values   map[string]string
		expected string
	}{
		{
			name:    "docker run simple",
			snippet: "snippet-with-complex-computed",
			values: map[string]string{
				"image_name": "nginx",
				"port":       "",
				"volume":     "",
				"detach":     "false",
			},
			expected: "docker run  nginx",
		},
		{
			name:    "docker run with port",
			snippet: "snippet-with-complex-computed",
			values: map[string]string{
				"image_name": "nginx",
				"port":       "8080:80",
				"volume":     "",
				"detach":     "false",
			},
			expected: "docker run -p 8080:80  nginx",
		},
		{
			name:    "docker run detached with all options",
			snippet: "snippet-with-complex-computed",
			values: map[string]string{
				"image_name": "nginx",
				"port":       "8080:80",
				"volume":     "/data:/app",
				"detach":     "true",
			},
			expected: "docker run -d -p 8080:80 -v /data:/app  nginx",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			snippet := config.Snippets[scenario.snippet]
			result, err := processor.ProcessSnippet(&snippet, scenario.values)
			if err != nil {
				t.Fatalf("Failed to process snippet: %v", err)
			}
			if result != scenario.expected {
				t.Errorf("Expected %q, got %q", scenario.expected, result)
			}
		})
	}
}

// TestEndToEnd_ValidationWorkflow tests validation workflows
func TestEndToEnd_ValidationWorkflow(t *testing.T) {
	config := loadTestConfig(t)
	processor := template.NewProcessor(config)

	t.Run("enum validation", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-enum"]

		// Valid values
		validValues := []string{"debug", "info", "warn", "error"}
		for _, val := range validValues {
			values := map[string]string{"log_level": val}
			_, err := processor.ProcessSnippet(&snippet, values)
			if err != nil {
				t.Errorf("Valid value %q should not error: %v", val, err)
			}
		}

		// Test with default
		result, err := processor.ProcessSnippet(&snippet, map[string]string{})
		if err != nil {
			t.Fatalf("Default value should work: %v", err)
		}
		if result != "app --log-level info" {
			t.Errorf("Expected default 'info', got %q", result)
		}
	})

	t.Run("range validation", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-range"]

		// Valid ports
		validPorts := []string{"1", "80", "8080", "65535"}
		for _, port := range validPorts {
			values := map[string]string{"port": port}
			_, err := processor.ProcessSnippet(&snippet, values)
			if err != nil {
				t.Errorf("Valid port %q should not error: %v", port, err)
			}
		}

		// Test with default
		result, err := processor.ProcessSnippet(&snippet, map[string]string{})
		if err != nil {
			t.Fatalf("Default port should work: %v", err)
		}
		if result != "server --port 8080" {
			t.Errorf("Expected default port '8080', got %q", result)
		}
	})

	t.Run("pattern validation", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-pattern"]

		// Valid versions
		validVersions := []string{"1.0.0", "v1.0.0", "2.3.4", "v10.20.30"}
		for _, version := range validVersions {
			values := map[string]string{"version": version}
			_, err := processor.ProcessSnippet(&snippet, values)
			if err != nil {
				t.Errorf("Valid version %q should not error: %v", version, err)
			}
		}
	})

	t.Run("regex type validation", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-regex-type"]

		// Valid regex patterns
		validPatterns := []string{`^test.*$`, `\d+`, `[a-z]+`}
		for _, pattern := range validPatterns {
			values := map[string]string{"pattern": pattern}
			_, err := processor.ProcessSnippet(&snippet, values)
			if err != nil {
				t.Errorf("Valid regex %q should not error: %v", pattern, err)
			}
		}
	})
}

// TestEndToEnd_ComputedVariablesWorkflow tests computed variables
func TestEndToEnd_ComputedVariablesWorkflow(t *testing.T) {
	config := loadTestConfig(t)
	processor := template.NewProcessor(config)

	t.Run("simple composition", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-computed-simple"]
		values := map[string]string{
			"resource_type": "pod",
			"resource_name": "my-pod",
		}
		result, err := processor.ProcessSnippet(&snippet, values)
		if err != nil {
			t.Fatalf("Failed to process snippet: %v", err)
		}
		if result != "app pod/my-pod" {
			t.Errorf("Expected 'app pod/my-pod', got %q", result)
		}
	})

	t.Run("conditional composition", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-computed-conditional"]

		// Same port
		values := map[string]string{
			"host_port":   "8080",
			"target_port": "",
		}
		result, err := processor.ProcessSnippet(&snippet, values)
		if err != nil {
			t.Fatalf("Failed to process snippet: %v", err)
		}
		if result != "server 8080:8080" {
			t.Errorf("Expected 'server 8080:8080', got %q", result)
		}

		// Different ports
		values = map[string]string{
			"host_port":   "8080",
			"target_port": "80",
		}
		result, err = processor.ProcessSnippet(&snippet, values)
		if err != nil {
			t.Fatalf("Failed to process snippet: %v", err)
		}
		if result != "server 8080:80" {
			t.Errorf("Expected 'server 8080:80', got %q", result)
		}
	})

	t.Run("complex composition with conditionals", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-complex-computed"]

		// All options
		values := map[string]string{
			"image_name": "nginx",
			"port":       "8080:80",
			"volume":     "/data:/app",
			"detach":     "true",
		}
		result, err := processor.ProcessSnippet(&snippet, values)
		if err != nil {
			t.Fatalf("Failed to process snippet: %v", err)
		}
		expected := "docker run -d -p 8080:80 -v /data:/app  nginx"
		if result != expected {
			t.Errorf("Expected %q, got %q", expected, result)
		}
	})
}

// TestEndToEnd_TransformWorkflow tests various transformations
func TestEndToEnd_TransformWorkflow(t *testing.T) {
	config := loadTestConfig(t)
	processor := template.NewProcessor(config)

	t.Run("boolean transforms", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-boolean"]

		testCases := []struct {
			verbose  string
			debug    string
			expected string
		}{
			{"false", "false", "app  "},
			{"true", "false", "app --verbose "},
			{"false", "true", "app  -d"},
			{"true", "true", "app --verbose -d"},
			{"yes", "false", "app --verbose "},
			{"1", "false", "app --verbose "},
		}

		for _, tc := range testCases {
			values := map[string]string{
				"verbose": tc.verbose,
				"debug":   tc.debug,
			}
			result, err := processor.ProcessSnippet(&snippet, values)
			if err != nil {
				t.Fatalf("Failed to process snippet: %v", err)
			}
			if result != tc.expected {
				t.Errorf("For verbose=%q debug=%q: expected %q, got %q",
					tc.verbose, tc.debug, tc.expected, result)
			}
		}
	})

	t.Run("value pattern transforms", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-value-pattern"]

		testCases := []struct {
			format   string
			expected string
		}{
			{"", "app "},
			{"json", "app --format=json"},
			{"yaml", "app --format=yaml"},
		}

		for _, tc := range testCases {
			values := map[string]string{"output_format": tc.format}
			result, err := processor.ProcessSnippet(&snippet, values)
			if err != nil {
				t.Fatalf("Failed to process snippet: %v", err)
			}
			if result != tc.expected {
				t.Errorf("For format=%q: expected %q, got %q",
					tc.format, tc.expected, result)
			}
		}
	})

	t.Run("empty value transforms", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-empty-value"]

		testCases := []struct {
			flag     string
			expected string
		}{
			{"", "app "},
			{"value", "app --flag=value"},
		}

		for _, tc := range testCases {
			values := map[string]string{"optional_flag": tc.flag}
			result, err := processor.ProcessSnippet(&snippet, values)
			if err != nil {
				t.Fatalf("Failed to process snippet: %v", err)
			}
			if result != tc.expected {
				t.Errorf("For flag=%q: expected %q, got %q",
					tc.flag, tc.expected, result)
			}
		}
	})
}

// TestEndToEnd_ComprehensiveSnippet tests the snippet with all features
func TestEndToEnd_ComprehensiveSnippet(t *testing.T) {
	config := loadTestConfig(t)
	processor := template.NewProcessor(config)
	snippet := config.Snippets["snippet-with-all-features"]

	scenarios := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name: "all features",
			values: map[string]string{
				"environment": "prod",
				"port":        "9000",
				"verbose":     "true",
				"log_level":   "debug",
				"extra_flag":  "test",
			},
			expected: "complex-app --env=prod --port=9000 --verbose --log=debug test",
		},
		{
			name: "minimal with defaults",
			values: map[string]string{
				"environment": "dev",
				"port":        "8080",
				"verbose":     "false",
				"log_level":   "",
				"extra_flag":  "",
			},
			expected: "complex-app --env=dev --port=8080 ",
		},
		{
			name: "staging environment",
			values: map[string]string{
				"environment": "staging",
				"port":        "8080",
				"verbose":     "true",
				"log_level":   "warn",
				"extra_flag":  "",
			},
			expected: "complex-app --env=staging --port=8080 --verbose --log=warn ",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			result, err := processor.ProcessSnippet(&snippet, scenario.values)
			if err != nil {
				t.Fatalf("Failed to process snippet: %v", err)
			}
			if result != scenario.expected {
				t.Errorf("Expected %q, got %q", scenario.expected, result)
			}
		})
	}
}

// TestEndToEnd_ErrorScenarios tests error handling in complete workflows
func TestEndToEnd_ErrorScenarios(t *testing.T) {
	config := loadTestConfig(t)

	t.Run("invalid enum value", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-enum"]

		for _, variable := range snippet.Variables {
			err := variable.ValidateWithConfig("invalid", config)
			if err == nil {
				t.Error("Expected validation error for invalid enum value")
			}
		}
	})

	t.Run("invalid range value", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-range"]

		for _, variable := range snippet.Variables {
			err := variable.ValidateWithConfig("99999", config)
			if err == nil {
				t.Error("Expected validation error for out of range value")
			}
		}
	})

	t.Run("invalid pattern value", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-pattern"]

		for _, variable := range snippet.Variables {
			err := variable.ValidateWithConfig("invalid", config)
			if err == nil {
				t.Error("Expected validation error for invalid pattern")
			}
		}
	})

	t.Run("invalid regex", func(t *testing.T) {
		snippet := config.Snippets["snippet-with-regex-type"]

		for _, variable := range snippet.Variables {
			err := variable.ValidateWithConfig("[unclosed", config)
			if err == nil {
				t.Error("Expected validation error for invalid regex")
			}
		}
	})
}
