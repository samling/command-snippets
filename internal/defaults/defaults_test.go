package defaults

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedSnippetsMatchTopLevelExamples(t *testing.T) {
	files := SnippetFiles()
	wantNames := []string{"docker.yaml", "git.yaml", "gnu.yaml", "kubernetes.yaml"}

	for _, name := range wantNames {
		t.Run(name, func(t *testing.T) {
			embedded, ok := files[name]
			if !ok {
				t.Fatalf("embedded snippet %q missing", name)
			}

			topLevel, err := os.ReadFile(filepath.Join("..", "..", "snippets", name))
			if err != nil {
				t.Fatalf("read top-level snippet: %v", err)
			}
			if !bytes.Equal(embedded, topLevel) {
				t.Fatalf("embedded snippet %q differs from top-level snippets/%s", name, name)
			}
		})
	}
}
