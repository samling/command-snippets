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

	if snippet.Command != "kubectl get pods ${namespace_arg}" {
		t.Fatalf("unexpected command %q", snippet.Command)
	}
	if len(snippet.Variables) != 2 {
		t.Fatalf("got %d variables, want 2", len(snippet.Variables))
	}
	if snippet.Variables[0].Name != "namespace_mode" {
		t.Fatalf("first variable = %q, want namespace_mode", snippet.Variables[0].Name)
	}
	if snippet.Variables[1].Name != "namespace" {
		t.Fatalf("second variable = %q, want namespace", snippet.Variables[1].Name)
	}
	if snippet.Variables[1].VisibleIf != `namespace_mode == "named"` {
		t.Fatalf("visible_if = %q", snippet.Variables[1].VisibleIf)
	}
	if snippet.Variables[1].RequiredIf != `namespace_mode == "named"` {
		t.Fatalf("required_if = %q", snippet.Variables[1].RequiredIf)
	}
	if _, ok := snippet.Computed["namespace_arg"]; !ok {
		t.Fatal("expected namespace_arg computed value")
	}

	result, err := snippet.ProcessTemplate(map[string]string{"namespace_mode": "named", "namespace": "default"}, nil)
	if err != nil {
		t.Fatalf("named namespace render failed: %v", err)
	}
	if result != "kubectl get pods -n default" {
		t.Fatalf("named namespace result = %q", result)
	}

	result, err = snippet.ProcessTemplate(map[string]string{"namespace_mode": "all"}, nil)
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
					if variable.Computed {
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
			values:  map[string]string{"namespace_mode": "named", "namespace": "default"},
			want:    "kubectl get pods -n default",
		},
		{
			name:    "docker optional flags",
			snippet: "docker-ps",
			values:  map[string]string{"show_all": "true", "filter": "name=myapp"},
			want:    "docker ps -a --filter name=myapp",
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

func loadShippedSnippetConfig(t *testing.T) *Config {
	t.Helper()

	cfg := &Config{
		Snippets:           make(map[string]Snippet),
		VariableTypes:      make(map[string]VariableType),
		TransformTemplates: make(map[string]TransformTemplate),
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
	}
	return cfg
}
