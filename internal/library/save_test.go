package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samling/command-snippets/internal/models"
)

func TestSaveIncludedEntryPreservesBytesCommentsAndIdentity(t *testing.T) {
	dir := t.TempDir()
	cwd := t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	source := filepath.Join(dir, "included.yaml")
	mainBytes := []byte("# main comment\nsettings:\n  sources: [included.yaml]\nsnippets: []\n")
	original := "# prefix\r\nsnippets:\r\n  # first entry\r\n  - name: First\r\n    # command help\r\n    command: |-\r\n      echo one\r\n      echo '${HOME}'\r\n\r\n  # sibling comment\r\n  - name: Second\r\n    command: echo two\r\n# trailing\r\n"
	if err := os.WriteFile(main, mainBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(original), 0640); err != nil {
		t.Fatal(err)
	}
	lib := Load(main, cwd)
	if err := lib.Error(); err != nil {
		t.Fatal(err)
	}
	entry := lib.Entries[0]
	draft := entry.Snippet
	draft.Name = "A friendly renamed command"
	next, saved, err := lib.Save(entry, draft, "")
	if err != nil {
		t.Fatal(err)
	}
	if !models.ValidID(saved.Snippet.ID) {
		t.Fatal("missing generated UUID")
	}
	if saved.Source.Path != source {
		t.Fatal("lost owner")
	}
	actual, _ := os.ReadFile(source)
	body := string(actual)
	suffix := "\r\n  # sibling comment\r\n  - name: Second\r\n    command: echo two\r\n# trailing\r\n"
	if !strings.HasPrefix(body, "# prefix\r\nsnippets:\r\n  # first entry\r\n") || !strings.HasSuffix(body, suffix) || !strings.Contains(body, "# command help") {
		t.Fatalf("unrelated bytes/comment lost:\n%q", body)
	}
	gotMain, _ := os.ReadFile(main)
	if string(gotMain) != string(mainBytes) {
		t.Fatal("main config changed")
	}
	info, _ := os.Stat(source)
	if info.Mode().Perm() != 0640 {
		t.Fatal("permissions changed")
	}
	if next.Entries[1].Snippet.ID != "" {
		t.Fatal("sibling assigned ID")
	}
	draft = saved.Snippet
	draft.Name = "Renamed again"
	_, savedAgain, err := next.Save(saved, draft, "")
	if err != nil || savedAgain.Snippet.ID != saved.Snippet.ID {
		t.Fatalf("rename ID: %v", err)
	}
}

func TestSaveFailuresKeepOriginal(t *testing.T) {
	for _, stage := range []string{"temp-write", "temp-sync", "temp-close", "replace", "before-replace"} {
		t.Run(stage, func(t *testing.T) {
			path := fixture(t, "config.yaml", "snippets:\n  - name: First\n    command: echo one\n")
			lib := Load(path, t.TempDir())
			before, _ := os.ReadFile(path)
			lib.saveHook = func(s, p string) error {
				if s == stage {
					return errors.New("injected " + stage)
				}
				return nil
			}
			entry := lib.Entries[0]
			draft := entry.Snippet
			draft.Name = "Changed"
			if _, _, err := lib.Save(entry, draft, ""); err == nil {
				t.Fatal("fault accepted")
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Fatal("original overwritten")
			}
			leftovers, _ := filepath.Glob(path + ".cs.lock")
			if len(leftovers) != 0 {
				t.Fatal("owned lock leaked")
			}
		})
	}
}
func TestSaveConflictsLocksAndCandidateValidation(t *testing.T) {
	path := fixture(t, "config.yaml", "snippets:\n  - name: First\n    command: echo one\n")
	lib := Load(path, t.TempDir())
	entry := lib.Entries[0]
	draft := entry.Snippet
	draft.Name = "Changed"
	external := []byte("snippets: []\n# external change\n")
	if err := os.WriteFile(path, external, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lib.Save(entry, draft, ""); err == nil {
		t.Fatal("external edit overwritten")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(external) {
		t.Fatal("external edit changed")
	}
	lib = Load(path, t.TempDir())
	draft = models.Snippet{Name: "New", Command: "echo {{missing}}"}
	if _, _, err := lib.Save(nil, draft, path); err == nil {
		t.Fatal("invalid template saved")
	}
	draft.Command = "echo ok"
	if err := os.WriteFile(path+".cs.lock", []byte("another writer"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lib.Save(nil, draft, path); err == nil {
		t.Fatal("lock stolen")
	}
	if _, err := os.Stat(path + ".cs.lock"); err != nil {
		t.Fatal("foreign lock removed")
	}
}
func TestSaveNewDestinationAndSettings(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	body := "# config\nsettings:\n  sources: ['*.snippets.yaml']\nsnippets: []\n# end\n"
	if err := os.WriteFile(main, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	lib := Load(main, t.TempDir())
	target := filepath.Join(dir, "new.snippets.yaml")
	next, entry, err := lib.Save(nil, models.Snippet{Name: "New friendly command", Command: "echo ok"}, target)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Snippet.ID == "" || len(next.Entries) != 1 {
		t.Fatal("new entry not discoverable")
	}
	if _, _, err := next.Save(nil, models.Snippet{Name: "Unconfigured", Command: "echo no"}, filepath.Join(dir, "outside.yaml")); err == nil {
		t.Fatal("unconfigured destination accepted")
	}
	settings := next.Settings
	settings.Color = "never"
	settings.Sources = nil
	final, err := next.SaveSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(final.Entries) != 0 {
		t.Fatal("source not removed")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("removed include deleted")
	}
	data, _ := os.ReadFile(main)
	if !strings.HasSuffix(string(data), "snippets: []\n# end\n") {
		t.Fatal("settings changed unrelated bytes")
	}
}

func TestSaveFreshAggregateConflictAndConcurrentCreation(t *testing.T) {
	dir := t.TempDir()
	main, include := filepath.Join(dir, "config.yaml"), filepath.Join(dir, "included.yaml")
	id := "12345678-1234-4234-8234-123456789abc"
	mainBody := "settings: {sources: [included.yaml, 'new-*.yaml']}\nsnippets:\n  - id: " + id + "\n    name: Owner\n    command: echo original\n"
	if err := os.WriteFile(main, []byte(mainBody), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(include, []byte("snippets: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lib := Load(main, t.TempDir())
	// Another source changes after the editor snapshot: candidate validation
	// must read it again instead of trusting the already-loaded aggregate.
	if err := os.WriteFile(include, []byte("snippets:\n  - id: "+id+"\n    name: Other\n    command: echo other\n"), 0600); err != nil {
		t.Fatal(err)
	}
	draft := lib.Entries[0].Snippet
	draft.Name = "Changed"
	if _, _, err := lib.Save(lib.Entries[0], draft, ""); err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("fresh aggregate conflict missed: %v", err)
	}
	if data, err := os.ReadFile(main); err != nil || string(data) != mainBody {
		t.Fatal("aggregate conflict overwrote owner")
	}
	if err := os.WriteFile(include, []byte("snippets: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"before-replace", "replace"} {
		t.Run(stage, func(t *testing.T) {
			lib := Load(main, t.TempDir())
			target := filepath.Join(dir, "new-"+stage+".yaml")
			external := []byte("snippets: []\n# concurrent creator\n")
			lib.saveHook = func(s, path string) error {
				if s == stage {
					return os.WriteFile(path, external, 0600)
				}
				return nil
			}
			if _, _, err := lib.Save(nil, models.Snippet{Name: "New", Command: "echo new"}, target); err == nil {
				t.Fatal("concurrent creator overwritten")
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != string(external) {
				t.Fatal("concurrent destination changed")
			}
			for _, pattern := range []string{target + ".cs.lock", filepath.Join(dir, ".cs-save-*")} {
				if paths, err := filepath.Glob(pattern); err != nil || len(paths) != 0 {
					t.Fatalf("attempt artifacts leaked: %v %v", paths, err)
				}
			}
		})
	}
	lib = Load(main, t.TempDir())
	settings := lib.Settings
	settings.Sources = append(settings.Sources, "missing.yaml")
	if _, err := lib.SaveSettings(settings); err == nil {
		t.Fatal("invalid settings candidate saved")
	}
	if data, err := os.ReadFile(main); err != nil || string(data) != mainBody {
		t.Fatal("invalid settings overwrote source")
	}
}
