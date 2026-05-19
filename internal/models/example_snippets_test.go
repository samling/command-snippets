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
