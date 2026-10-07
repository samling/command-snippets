package models

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestCanonicalConfigYAML(t *testing.T) {
	cfg := Config{Settings: DefaultSettings()}
	if err := yaml.Unmarshal([]byte("snippets:\n  - name: List pods\n    command: kubectl get pods\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Snippets) != 1 || cfg.Snippets[0].Name != "List pods" || cfg.Settings.Color != "auto" || !cfg.Settings.ProjectSource {
		t.Fatalf("config %+v", cfg)
	}
}
