package defaults

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedSnippetsMatchTopLevelExamples(t *testing.T) {
	files := SnippetFiles()
	topLevelPaths, err := filepath.Glob(filepath.Join("..", "..", "snippets", "*.yaml"))
	if err != nil {
		t.Fatalf("discover top-level snippets: %v", err)
	}

	wantNames := make(map[string]struct{}, len(topLevelPaths))
	for _, path := range topLevelPaths {
		name := filepath.Base(path)
		wantNames[name] = struct{}{}

		t.Run(name, func(t *testing.T) {
			embedded, ok := files[name]
			if !ok {
				t.Fatalf("embedded snippet %q missing", name)
			}

			topLevel, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read top-level snippet: %v", err)
			}
			if !bytes.Equal(embedded, topLevel) {
				t.Fatalf("embedded snippet %q differs from top-level snippets/%s", name, name)
			}
		})
	}

	for name := range files {
		if _, ok := wantNames[name]; ok {
			continue
		}
		t.Fatalf("embedded snippet %q has no top-level snippets/%s counterpart", name, name)
	}
}
