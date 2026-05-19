package defaults

import (
	"embed"
	"path/filepath"
	"sort"
)

//go:embed snippets/*.yaml
var snippetFS embed.FS

func SnippetFiles() map[string][]byte {
	entries, err := snippetFS.ReadDir("snippets")
	if err != nil {
		return nil
	}

	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := snippetFS.ReadFile(filepath.Join("snippets", name))
		if err != nil {
			continue
		}
		files[name] = data
	}
	return files
}

func SnippetFileNames() []string {
	files := SnippetFiles()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
