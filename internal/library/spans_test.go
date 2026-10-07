package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samling/command-snippets/internal/models"
)

func TestSaveMetadataAndLateExternalConflicts(t *testing.T) {
	for _, change := range []string{"inode", "deleted", "permissions", "symlink", "late-edit", "readonly"} {
		t.Run(change, func(t *testing.T) {
			path := fixture(t, "config.yaml", "snippets:\n  - name: Owner\n    command: echo original\n")
			if change == "readonly" {
				if err := os.Chmod(path, 0400); err != nil {
					t.Fatal(err)
				}
			}
			lib := Load(path, t.TempDir())
			entry := lib.Entries[0]
			draft := entry.Snippet
			draft.Name = "Changed"
			switch change {
			case "inode":
				other := path + ".new"
				if err := os.WriteFile(other, entry.Source.Bytes, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(other, path); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "permissions":
				if err := os.Chmod(path, 0640); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := path + ".target"
				if err := os.WriteFile(target, entry.Source.Bytes, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "late-edit":
				lib.saveHook = func(stage, p string) error {
					if stage == "before-replace" {
						return os.WriteFile(path, []byte("snippets: []\n# external\n"), 0600)
					}
					return nil
				}
			}
			if _, _, err := lib.Save(entry, draft, ""); err == nil {
				t.Fatal("conflict or read-only source accepted")
			}
			if change != "deleted" {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), "Changed") {
					t.Fatal("original replaced")
				}
			}
		})
	}
}
func TestDirectorySyncFailureRefreshesSnapshotAndKeepsID(t *testing.T) {
	path := fixture(t, "config.yaml", "snippets: []\n")
	lib := Load(path, t.TempDir())
	lib.saveHook = func(stage, p string) error {
		if stage == "directory-sync" {
			return errors.New("injected durability failure")
		}
		return nil
	}
	next, entry, err := lib.Save(nil, models.Snippet{Name: "Saved", Command: "echo ok"}, path)
	if err == nil || next == nil || entry == nil || !strings.Contains(err.Error(), "saved") {
		t.Fatal("post-publication state not returned")
	}
	id := entry.Snippet.ID
	next, entry, err = next.Save(entry, entry.Snippet, "")
	if err != nil || len(next.Entries) != 1 || entry.Snippet.ID != id {
		t.Fatalf("retry duplicated or changed identity: %v", err)
	}
}
func TestBOMInlineCommentsAndNoFinalNewline(t *testing.T) {
	path := fixture(t, "config.yaml", "\ufeffsettings: {color: auto}\nsnippets: [] # keep this note\n")
	lib := Load(path, t.TempDir())
	next, _, err := lib.Save(nil, models.Snippet{Name: "First", Command: "echo one"}, path)
	if err != nil {
		t.Fatal(err)
	}
	settings := next.Settings
	settings.Color = "never"
	_, err = next.SaveSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), "\ufeff") || !strings.Contains(string(data), "# keep this note") {
		t.Fatalf("BOM/comment lost: %q %v", data, err)
	}
	path = fixture(t, "no-newline.yaml", "snippets:\n  - name: First\n    command: echo one")
	lib = Load(path, t.TempDir())
	next, _, err = lib.Save(nil, models.Snippet{Name: "Second", Command: "echo two"}, path)
	if err != nil || len(next.Entries) != 2 {
		t.Fatalf("append without newline: %v", err)
	}
}

func TestProjectOwnerAndTypedScalarRejections(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	local := filepath.Join(dir, ".csnippets")
	mainBody := []byte("snippets: []\n")
	if err := os.WriteFile(main, mainBody, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("# project\nsnippets:\n  - name: Local\n    command: >-\n      echo one\n      two\n\n  # sibling\n  - name: Sibling\n    command: echo sibling\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lib := Load(main, dir)
	draft := lib.Entries[0].Snippet
	draft.Name = "Local rename"
	_, _, err := lib.Save(lib.Entries[0], draft, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(main)
	if string(data) != string(mainBody) {
		t.Fatal("project edit flattened")
	}
	data, _ = os.ReadFile(local)
	if !strings.Contains(string(data), "# sibling\n  - name: Sibling\n    command: echo sibling\n") {
		t.Fatal("folded scalar boundary changed sibling")
	}
	for _, body := range []string{"snippets:\n  - name: 123\n    command: echo ok\n", "snippets:\n  - name: Bad\n    command: true\n", "settings: {project_source: 'true'}\nsnippets: []\n", "{snippets: []}"} {
		path := fixture(t, "invalid.yaml", body)
		if Load(path, t.TempDir()).Error() == nil {
			t.Fatalf("typed/unsafe YAML accepted: %s", body)
		}
	}
}
