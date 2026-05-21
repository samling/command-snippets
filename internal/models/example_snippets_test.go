package models

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKubernetesConditionalNamespaceExample(t *testing.T) {
	path := filepath.Join("..", "..", "snippets", "kubernetes.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kubernetes snippets: %v", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse kubernetes snippets: %v", err)
	}

	snippet, ok := cfg.Snippets["kubectl-get-pods-conditional-namespace"]
	if !ok {
		t.Fatal("expected conditional namespace example snippet")
	}

	if snippet.Command != "kubectl get pods ${namespace}" {
		t.Fatalf("unexpected command %q", snippet.Command)
	}
	if len(snippet.Variables) != 1 {
		t.Fatalf("got %d variables, want 1", len(snippet.Variables))
	}
	if snippet.Variables[0].Name != "namespace" {
		t.Fatalf("variable = %q, want namespace", snippet.Variables[0].Name)
	}
	if snippet.Variables[0].ComputedTemplate != "namespace" {
		t.Fatalf("computed_template = %q, want namespace", snippet.Variables[0].ComputedTemplate)
	}
	if _, ok := cfg.ComputedTemplates["namespace"]; !ok {
		t.Fatal("expected namespace computed template")
	}

	result, err := snippet.ProcessTemplate(map[string]string{"namespace": "default"}, &cfg)
	if err != nil {
		t.Fatalf("named namespace render failed: %v", err)
	}
	if result != "kubectl get pods -n default" {
		t.Fatalf("named namespace result = %q", result)
	}

	result, err = snippet.ProcessTemplate(map[string]string{"namespace": "all"}, &cfg)
	if err != nil {
		t.Fatalf("all namespace render failed: %v", err)
	}
	if result != "kubectl get pods -A" {
		t.Fatalf("all namespace result = %q", result)
	}
}

func TestShippedSnippetsUseCurrentTemplateStyle(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "snippets", "*.yaml"))
	if err != nil {
		t.Fatalf("glob shipped snippets: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected shipped snippet files")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			if len(cfg.TransformTemplates) > 0 {
				t.Fatalf("%s defines transform_templates; shipped examples should use current computed syntax", path)
			}

			for snippetName, snippet := range cfg.Snippets {
				for _, variable := range snippet.Variables {
					if variable.TransformTemplate != "" {
						t.Fatalf("%s/%s variable %s uses transform_template", path, snippetName, variable.Name)
					}
					if variable.Transform != nil {
						t.Fatalf("%s/%s variable %s uses legacy transform", path, snippetName, variable.Name)
					}
					if variable.Computed.IsLegacy() {
						t.Fatalf("%s/%s variable %s uses legacy computed variable", path, snippetName, variable.Name)
					}
				}
			}
		})
	}
}

func TestShippedCurrentStyleSnippetRendering(t *testing.T) {
	cfg := loadShippedSnippetConfig(t)

	tests := []struct {
		name    string
		snippet string
		values  map[string]string
		want    string
	}{
		{
			name:    "kubernetes namespace mode",
			snippet: "kubectl-get-pods",
			values:  map[string]string{"namespace": "default"},
			want:    "kubectl get pods -n default",
		},
		{
			name:    "kubernetes all namespaces",
			snippet: "kubectl-get-pods",
			values:  map[string]string{"namespace": "all"},
			want:    "kubectl get pods -A",
		},
		{
			name:    "kubernetes empty namespace",
			snippet: "kubectl-get-pods",
			values:  map[string]string{"namespace": ""},
			want:    "kubectl get pods",
		},
		{
			name:    "docker optional flags",
			snippet: "docker-ps",
			values:  map[string]string{"show_all": "true", "filter": "name=myapp"},
			want:    "docker ps -a --filter name=myapp",
		},
		{
			name:    "docker advanced repeats env flag",
			snippet: "docker-run-advanced",
			values:  map[string]string{"image_name": "nginx", "env_var": "TEST=TEST FOO=BAR"},
			want:    "docker run -e TEST=TEST -e FOO=BAR nginx",
		},
		{
			name:    "git computed message flag",
			snippet: "git-commit-amend",
			values:  map[string]string{"no_edit": "false", "new_message": "fix bug"},
			want:    "git commit --amend -m 'fix bug'",
		},
		{
			name:    "gnu sed global flag",
			snippet: "sed-replace",
			values:  map[string]string{"search_pattern": "foo", "replacement": "bar", "global": "true", "file": "app.txt"},
			want:    "sed -E 's/foo/bar/g' app.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snippet, ok := cfg.Snippets[tt.snippet]
			if !ok {
				t.Fatalf("snippet %q not found", tt.snippet)
			}
			got, err := snippet.ProcessTemplate(tt.values, cfg)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDockerRunAdvancedAllowsRepeatedEnvVars(t *testing.T) {
	cfg := loadShippedSnippetConfig(t)
	snippet, ok := cfg.Snippets["docker-run-advanced"]
	if !ok {
		t.Fatal("snippet docker-run-advanced not found")
	}

	for _, variable := range snippet.Variables {
		if variable.Name != "env_var" {
			continue
		}
		if err := variable.ValidateWithConfig("test=test foo=bar", cfg); err != nil {
			t.Fatalf("env_var validation rejected repeated env vars: %v", err)
		}
		return
	}

	t.Fatal("env_var variable not found")
}

func loadShippedSnippetConfig(t *testing.T) *Config {
	t.Helper()

	cfg := &Config{
		Snippets:           make(map[string]Snippet),
		VariableTypes:      make(map[string]VariableType),
		TransformTemplates: make(map[string]TransformTemplate),
		ComputedTemplates:  make(map[string]ComputedTemplate),
	}
	paths, err := filepath.Glob(filepath.Join("..", "..", "snippets", "*.yaml"))
	if err != nil {
		t.Fatalf("glob shipped snippets: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var partial Config
		if err := yaml.Unmarshal(data, &partial); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for name, snippet := range partial.Snippets {
			cfg.Snippets[name] = snippet
		}
		for name, variableType := range partial.VariableTypes {
			cfg.VariableTypes[name] = variableType
		}
		for name, transformTemplate := range partial.TransformTemplates {
			cfg.TransformTemplates[name] = transformTemplate
		}
		for name, computedTemplate := range partial.ComputedTemplates {
			cfg.ComputedTemplates[name] = computedTemplate
		}
	}
	return cfg
}
