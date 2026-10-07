package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadFriendlyNamesIdentityAndReadOnly(t *testing.T) {
	p := fixture(t, "config.yaml", "snippets:\n  - name: Pod resource usage, sorted\n    command: echo first\n  - name: Pod resource usage, sorted\n    command: echo second\n")
	before, _ := os.ReadFile(p)
	lib := Load(p, t.TempDir())
	if err := lib.Error(); err != nil {
		t.Fatal(err)
	}
	if len(lib.Entries) != 2 || lib.Entries[0].Key == lib.Entries[1].Key {
		t.Fatalf("entries: %+v", lib.Entries)
	}
	if lib.Entries[0].Snippet.ID != "" {
		t.Fatal("load assigned a persisted ID")
	}
	after, _ := os.ReadFile(p)
	if string(after) != string(before) {
		t.Fatal("load wrote the source")
	}
	if !lib.Settings.ProjectSource || lib.Settings.Color != "auto" {
		t.Fatalf("defaults: %+v", lib.Settings)
	}
}

func TestLoadStrictSchema(t *testing.T) {
	for name, body := range map[string]string{
		"legacy":             "snippets:\n  slug:\n    command: echo old\n",
		"unknown":            "snippets:\n  - name: Example\n    command: echo ok\n    created_at: yesterday\n",
		"duplicate":          "snippets:\n  - name: One\n    name: Two\n    command: echo ok\n",
		"alias":              "snippets:\n  - &one {name: One, command: echo ok}\n  - *one\n",
		"multiple documents": "snippets: []\n---\nsnippets: []\n",
		"bad id":             "snippets:\n  - id: not-a-uuid\n    name: One\n    command: echo ok\n",
		"recursive include":  "sources: [nested.yaml]\nsnippets: []\n",
		"custom tag":         "snippets:\n  - name: !custom One\n    command: echo ok\n",
		"merge key":          "snippets:\n  - name: One\n    command: echo ok\n    <<: {description: merged}\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := fixture(t, "config.yaml", body)
			lib := Load(p, t.TempDir())
			if lib.Error() == nil {
				t.Fatal("invalid source accepted")
			}
			if !strings.Contains(lib.Error().Error(), p) {
				t.Fatal("missing source location")
			}
			if after, err := os.ReadFile(p); err != nil || string(after) != body {
				t.Fatal("invalid source was rewritten")
			}
		})
	}
}

func TestLoadIncludesDuplicateIDsAndLocal(t *testing.T) {
	dir := t.TempDir()
	cwd := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(p, "settings:\n  sources: [included.yaml, 'missing*.yaml']\nsnippets:\n  - name: Main\n    command: echo main\n")
	include := filepath.Join(dir, "included.yaml")
	write(include, "snippets:\n  - name: Included\n    command: echo included\n")
	write(filepath.Join(cwd, ".csnippets"), "snippets:\n  - name: Local\n    command: echo local\n")
	lib := Load(p, cwd)
	if err := lib.Error(); err != nil {
		t.Fatal(err)
	}
	if len(lib.Entries) != 3 || len(lib.Warnings) != 1 {
		t.Fatalf("entries/warnings: %d/%v", len(lib.Entries), lib.Warnings)
	}
	if lib.Entries[1].Source.Path != include || lib.Entries[2].Source.Kind != "project" {
		t.Fatal("provenance lost")
	}
	id := "12345678-1234-4234-8234-123456789abc"
	write(include, "snippets:\n  - id: "+id+"\n    name: First\n    command: echo one\n  - id: "+id+"\n    name: Second\n    command: echo two\n")
	lib = Load(p, cwd)
	if lib.Error() == nil || !strings.Contains(lib.Error().Error(), "duplicate id") {
		t.Fatalf("duplicate ID: %v", lib.Error())
	}
	write(include, "settings: {}\nsnippets: []\n")
	if Load(p, cwd).Error() == nil {
		t.Fatal("included settings accepted")
	}
}

func TestLoadOrderingAliasesEmptySourcesAndCrossSourceIDs(t *testing.T) {
	dir, cwd := t.TempDir(), t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(main, "settings:\n  sources: ['part-*.yaml', part-a.yaml, alias.yaml, empty.yaml]\nsnippets:\n  - name: Main\n    command: echo main\n")
	a, b := filepath.Join(dir, "part-a.yaml"), filepath.Join(dir, "part-b.yaml")
	write(b, "snippets:\n  - name: B\n    command: echo b\n")
	write(a, "snippets:\n  - name: A\n    command: echo a\n")
	write(filepath.Join(dir, "empty.yaml"), "# empty included source\n")
	write(filepath.Join(cwd, ".csnippets"), "snippets:\n  - name: Local\n    command: echo local\n")
	if err := os.Symlink(a, filepath.Join(dir, "alias.yaml")); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(main)
	for i := 0; i < 2; i++ {
		lib := Load(main, cwd)
		if err := lib.Error(); err != nil {
			t.Fatal(err)
		}
		if len(lib.Entries) != 4 || len(lib.Sources) != 5 {
			t.Fatalf("sources/entries duplicated: %d/%d", len(lib.Sources), len(lib.Entries))
		}
		for j, name := range []string{"Main", "A", "B", "Local"} {
			if lib.Entries[j].Snippet.Name != name || lib.Entries[j].Snippet.ID != "" {
				t.Fatalf("order/identity at %d: %+v", j, lib.Entries[j])
			}
		}
	}
	if after, err := os.ReadFile(main); err != nil || string(after) != string(before) {
		t.Fatal("repeated load changed main source")
	}
	id := "12345678-1234-4234-8234-123456789abc"
	write(a, "snippets:\n  - id: "+id+"\n    name: Same\n    command: echo a\n")
	write(b, "snippets:\n  - id: "+id+"\n    name: Same\n    command: echo b\n")
	lib := Load(main, cwd)
	if err := lib.Error(); err == nil || !strings.Contains(err.Error(), a+":2") || !strings.Contains(err.Error(), b+":2") {
		t.Fatalf("duplicate must identify both source lines: %v", lib.Error())
	}
	write(a, "snippets: []\n")
	write(b, "snippets: []\n")
	write(filepath.Join(cwd, ".csnippets"), "snippets: []\n")
	write(main, "settings: {sources: ['part-*.yaml']}\nsnippets: []\n")
	if lib := Load(main, cwd); lib.Error() != nil || len(lib.Entries) != 0 {
		t.Fatalf("empty libraries invalid: %v", lib.Error())
	}
	write(main, "settings: {sources: [not-a-file]}\nsnippets: []\n")
	if err := os.Mkdir(filepath.Join(dir, "not-a-file"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := Load(main, cwd).Error(); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("non-file source accepted: %v", err)
	}
}
