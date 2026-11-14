package models_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
)

func TestShippedCanonicalExamplesValidateAndRenderPodGolden(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "snippets", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	type renderCase struct {
		values map[string]any
		want   string
	}
	cases := map[string]renderCase{
		"Run a container with options":   {map[string]any{"detach": true, "env": []string{"A=1", "B=hello world"}, "name": "web app"}, "docker run -d -e A=1 -e 'B=hello world' --name 'web app' nginx:latest"},
		"Follow container logs":          {map[string]any{"container": "web"}, "docker logs -f web --tail 100"},
		"Git status":                     {nil, "git status --short"},
		"Commit staged changes":          {map[string]any{"message": "Fix pods"}, "git commit -m 'Fix pods'"},
		"Find commits by message":        {map[string]any{"pattern": "fix.*"}, "git log --oneline --grep 'fix.*'"},
		"Find text in files":             {map[string]any{"pattern": "pod.*"}, "grep -r -E 'pod.*' ."},
		"List directory sizes":           {nil, "du -h --max-depth=1 . | sort -h"},
		"List pods":                      {map[string]any{"namespace": "all"}, "kubectl get pods -A"},
		"Get pods in a chosen namespace": {map[string]any{"mode": "Named", "namespace": "team-a"}, "kubectl get pods  -n team-a"},
		"Forward a service port":         {map[string]any{"service": "web"}, "kubectl port-forward 'svc/web' '8080:8080' "},
		"Search with an optional limit":  {map[string]any{"pattern": "pod.*", "limit": "10"}, "grep -E 'pod.*' . -m 10"},
		"Echo a literal shell variable":  {nil, `echo '${HOME}' "$HOME" '{{example}}'`},
	}
	ids := map[string]bool{}
	for _, path := range paths {
		path, err = filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		lib := library.Load(path, t.TempDir())
		if err := lib.Error(); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, entry := range lib.Entries {
			if !models.ValidID(entry.Snippet.ID) || ids[entry.Snippet.ID] {
				t.Fatalf("missing, invalid or duplicate bundled identity: %s", entry.Snippet.Name)
			}
			ids[entry.Snippet.ID] = true
			if entry.Snippet.Name != "Pod resource usage, sorted" {
				tc, ok := cases[entry.Snippet.Name]
				if !ok {
					t.Fatalf("bundled example lacks representative render case: %s", entry.Snippet.Name)
				}
				command, err := entry.Template.Render(tc.values)
				if err != nil || command != tc.want {
					t.Fatalf("%s: got %q want %q: %v", entry.Snippet.Name, command, tc.want, err)
				}
				delete(cases, entry.Snippet.Name)
			}
			if entry.Snippet.Name == "Pod resource usage, sorted" {
				if len(entry.Snippet.Expressions) != 0 {
					t.Fatal("pod example requires expressions")
				}
				for _, tc := range []struct{ namespace, sort, key, golden string }{{"", "CPU", "3", "pods-current-cpu.txt"}, {"team-a", "Memory", "4", "pods-named-memory.txt"}, {"all", "Memory", "4", "pods-all-memory.txt"}} {
					command, err := entry.Template.Render(map[string]any{"namespace": tc.namespace, "sort_by": tc.sort})
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(command, "sort -k"+tc.key+","+tc.key) || !strings.Contains(command, `print "-", $0`) || !strings.Contains(command, `"$header"`) {
						t.Fatalf("pod golden %q", command)
					}
					golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", tc.golden))
					if err != nil {
						t.Fatal(err)
					}
					if command != string(golden) {
						t.Fatalf("%s: got %q want %q", tc.golden, command, golden)
					}
					before, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(before), "{{sort_by}}") {
						t.Fatal("render changed example")
					}
				}
			}
		}
	}
	if len(cases) != 0 {
		t.Fatalf("render cases reference missing examples: %v", cases)
	}
}
