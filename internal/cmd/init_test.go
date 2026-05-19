package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	path, err := defaultConfigPath()
	if err != nil {
		t.Fatalf("defaultConfigPath returned error: %v", err)
	}
	if path != filepath.Join("/tmp/xdg-config", "cs", "config.yaml") {
		t.Fatalf("path = %q, want XDG config path", path)
	}
}

func TestMissingConfigErrorMentionsInit(t *testing.T) {
	err := missingConfigError(filepath.Join("/tmp", "missing", "config.yaml"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cs init") {
		t.Fatalf("error %q should mention cs init", err)
	}
}

func TestRunInitUsesConfiguredPath(t *testing.T) {
	dir := t.TempDir()
	oldCfgFile := cfgFile
	t.Cleanup(func() { cfgFile = oldCfgFile })
	cfgFile = filepath.Join(dir, "config.yaml")

	if _, err := runInit(initOptions{}); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("config was not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "snippets", "docker.yaml")); err != nil {
		t.Fatalf("docker snippet was not created: %v", err)
	}
}

func TestRunInitRejectsForceAndMissingTogether(t *testing.T) {
	if _, err := runInit(initOptions{Force: true, Missing: true}); err == nil {
		t.Fatal("expected force plus missing to return error")
	}
}

func TestPrintInitResultReportsCreatedFiles(t *testing.T) {
	var out bytes.Buffer
	configPath := filepath.Join("tmp", "cs", "config.yaml")
	result := initResult{
		Created: []string{
			configPath,
			filepath.Join("tmp", "cs", "snippets", "docker.yaml"),
		},
	}

	printInitResult(&out, configPath, result)
	got := out.String()

	assertContains(t, got, "Initialized CS config at "+configPath)
	assertContains(t, got, "Created:")
	assertContains(t, got, filepath.Join("tmp", "cs", "snippets", "docker.yaml"))
}

func TestPrintInitResultReportsNoChangesAndSuggestions(t *testing.T) {
	var out bytes.Buffer
	configPath := filepath.Join("tmp", "cs", "config.yaml")
	result := initResult{
		Skipped: []string{
			configPath,
			filepath.Join("tmp", "cs", "snippets", "docker.yaml"),
		},
	}

	printInitResult(&out, configPath, result)
	got := out.String()

	assertContains(t, got, "CS config already exists at "+configPath)
	assertContains(t, got, "Nothing changed.")
	assertContains(t, got, "cs init --missing")
	assertContains(t, got, "cs init --force")
}

func TestPrintInitResultReportsMixedCreatedAndSkippedFiles(t *testing.T) {
	var out bytes.Buffer
	configPath := filepath.Join("tmp", "cs", "config.yaml")
	result := initResult{
		Created: []string{filepath.Join("tmp", "cs", "snippets", "git.yaml")},
		Skipped: []string{configPath},
	}

	printInitResult(&out, configPath, result)
	got := out.String()

	assertContains(t, got, "Created:")
	assertContains(t, got, filepath.Join("tmp", "cs", "snippets", "git.yaml"))
	assertContains(t, got, "Skipped existing:")
	assertContains(t, got, configPath)
	assertContains(t, got, "cs init --force")
}

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

func assertContains(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("output did not contain %q:\n%s", want, got)
	}
}
