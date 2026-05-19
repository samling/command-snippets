package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteInitialConfigCreatesConfigAndSnippets(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	snippetFiles := map[string][]byte{
		"git.yaml":    []byte("git snippets"),
		"docker.yaml": []byte("docker snippets"),
	}

	result, err := writeInitialConfig(configPath, snippetFiles, initOptions{})
	if err != nil {
		t.Fatalf("writeInitialConfig() error = %v", err)
	}

	assertFileContent(t, configPath, mustDefaultConfigYAML(t))
	assertFileContent(t, filepath.Join(dir, "snippets", "git.yaml"), []byte("git snippets"))
	assertFileContent(t, filepath.Join(dir, "snippets", "docker.yaml"), []byte("docker snippets"))
	assertPaths(t, result.Created,
		configPath,
		filepath.Join(dir, "snippets", "docker.yaml"),
		filepath.Join(dir, "snippets", "git.yaml"),
	)
	assertPaths(t, result.Skipped)
	assertPaths(t, result.Overwritten)
}

func TestWriteInitialConfigDoesNotOverwriteExistingFiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	snippetPath := filepath.Join(dir, "snippets", "git.yaml")
	mustWriteFile(t, configPath, []byte("existing config"))
	mustWriteFile(t, snippetPath, []byte("existing snippets"))

	result, err := writeInitialConfig(configPath, map[string][]byte{
		"git.yaml": []byte("new snippets"),
	}, initOptions{})
	if err != nil {
		t.Fatalf("writeInitialConfig() error = %v", err)
	}

	assertFileContent(t, configPath, []byte("existing config"))
	assertFileContent(t, snippetPath, []byte("existing snippets"))
	assertPaths(t, result.Created)
	assertPaths(t, result.Skipped, configPath, snippetPath)
	assertPaths(t, result.Overwritten)
}

func TestWriteInitialConfigMissingRestoresOnlyMissingSnippets(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	existingSnippetPath := filepath.Join(dir, "snippets", "git.yaml")
	missingSnippetPath := filepath.Join(dir, "snippets", "docker.yaml")
	mustWriteFile(t, configPath, []byte("existing config"))
	mustWriteFile(t, existingSnippetPath, []byte("existing snippets"))

	result, err := writeInitialConfig(configPath, map[string][]byte{
		"git.yaml":    []byte("new git snippets"),
		"docker.yaml": []byte("docker snippets"),
	}, initOptions{Missing: true})
	if err != nil {
		t.Fatalf("writeInitialConfig() error = %v", err)
	}

	assertFileContent(t, configPath, []byte("existing config"))
	assertFileContent(t, existingSnippetPath, []byte("existing snippets"))
	assertFileContent(t, missingSnippetPath, []byte("docker snippets"))
	assertPaths(t, result.Created, missingSnippetPath)
	assertPaths(t, result.Skipped, configPath, existingSnippetPath)
	assertPaths(t, result.Overwritten)
}

func TestWriteInitialConfigForceOverwritesFiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	snippetPath := filepath.Join(dir, "snippets", "git.yaml")
	mustWriteFile(t, configPath, []byte("existing config"))
	mustWriteFile(t, snippetPath, []byte("existing snippets"))

	result, err := writeInitialConfig(configPath, map[string][]byte{
		"git.yaml": []byte("new snippets"),
	}, initOptions{Force: true})
	if err != nil {
		t.Fatalf("writeInitialConfig() error = %v", err)
	}

	assertFileContent(t, configPath, mustDefaultConfigYAML(t))
	assertFileContent(t, snippetPath, []byte("new snippets"))
	assertPaths(t, result.Created)
	assertPaths(t, result.Skipped)
	assertPaths(t, result.Overwritten, configPath, snippetPath)
}

func TestWriteTrackedFileMissingSkipsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	mustWriteFile(t, path, []byte("existing config"))

	var result initResult
	if err := writeTrackedFile(path, []byte("new config"), initOptions{Missing: true}, &result); err != nil {
		t.Fatalf("writeTrackedFile() error = %v", err)
	}

	assertFileContent(t, path, []byte("existing config"))
	assertPaths(t, result.Created)
	assertPaths(t, result.Skipped, path)
	assertPaths(t, result.Overwritten)
}

func mustDefaultConfigYAML(t *testing.T) []byte {
	t.Helper()
	data, err := defaultConfigYAML()
	if err != nil {
		t.Fatalf("defaultConfigYAML() error = %v", err)
	}
	return data
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

func assertPaths(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	seen := make(map[string]int, len(got))
	for _, path := range got {
		seen[path]++
	}
	for _, path := range want {
		if seen[path] == 0 {
			t.Fatalf("paths = %v, missing %q", got, path)
		}
		seen[path]--
	}
	for path, count := range seen {
		if count != 0 {
			t.Fatalf("paths = %v, unexpected %q", got, path)
		}
	}
}
