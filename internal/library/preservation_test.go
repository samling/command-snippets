package library

import (
	"os"
	"strings"
	"testing"

	"github.com/samling/command-snippets/internal/models"
)

func TestKeepChompRenameAndAppendPreserveCommand(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, style := range []string{"|+", ">+", "|2+", ">+2"} {
			for _, suffix := range []string{"", "  # sibling note\n  - name: Sibling\n    command: echo sibling\n", "# outside\nsettings: {color: auto}\n...\n# tail\n"} {
				t.Run(style+"/"+map[bool]string{true: "CRLF", false: "LF"}[newline == "\r\n"]+"/"+suffix, func(t *testing.T) {
					body := "snippets:\n  - name: Original\n    command: " + style + "\n      echo original\n      second line\n\n\n" + suffix
					body = strings.ReplaceAll(body, "\n", newline)
					path := fixture(t, "config.yaml", body)
					lib := Load(path, t.TempDir())
					if err := lib.Error(); err != nil {
						t.Fatal(err)
					}
					want := lib.Entries[0].Snippet.Command
					for i := 0; i < 3; i++ {
						entry := lib.Entries[0]
						draft := entry.Snippet
						draft.Name += " renamed"
						var err error
						lib, _, err = lib.Save(entry, draft, "")
						if err != nil {
							t.Fatal(err)
						}
						if got := lib.Entries[0].Snippet.Command; got != want {
							t.Fatalf("rename %d: want %q got %q", i, want, got)
						}
					}
					renamed, err := os.ReadFile(path)
					if err != nil || !strings.HasSuffix(string(renamed), strings.ReplaceAll(suffix, "\n", newline)) {
						t.Fatalf("rename changed unrelated suffix: %q %v", renamed, err)
					}
					lib, _, err = lib.Save(nil, models.Snippet{Name: "Added", Command: "echo safe"}, path)
					if err != nil {
						t.Fatal(err)
					}
					if got := lib.Entries[0].Snippet.Command; got != want {
						t.Fatalf("append: want %q got %q", want, got)
					}
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(data), strings.ReplaceAll(suffix, "\n", newline)) {
						t.Fatalf("unrelated suffix changed: %q", data)
					}
				})
			}
		}
	}
}

func TestRemovedSubtreeCommentsSurviveOnceInOrder(t *testing.T) {
	for _, removeField := range []bool{false, true} {
		t.Run(map[bool]string{false: "item", true: "field"}[removeField], func(t *testing.T) {
			suffix := "  # outside sibling\n  - name: Sibling\n    command: echo sibling\n# outside tail\n"
			body := "snippets:\n  - name: Original\n    command: echo {{keep}}\n    inputs:\n      - name: keep\n      # first removed note\n      - name: gone\n        help: obsolete # second removed note\n        special:\n          value: output # third removed note\n        kind: choice\n        choices: # fourth removed note\n          # fifth removed note\n          - label: Old\n            value: output # sixth removed note\n" + suffix
			path := fixture(t, "config.yaml", body)
			lib := Load(path, t.TempDir())
			if err := lib.Error(); err != nil {
				t.Fatal(err)
			}
			entry := lib.Entries[0]
			draft := entry.Snippet
			draft.Inputs = draft.Inputs[:1]
			if removeField {
				draft.Inputs = nil
				draft.Command = "echo safe"
			}
			var err error
			lib, _, err = lib.Save(entry, draft, "")
			if err != nil {
				t.Fatal(err)
			}
			var previous []byte
			for i := 0; i < 3; i++ {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				last := -1
				for _, note := range []string{"# first removed note", "# second removed note", "# third removed note", "# fourth removed note", "# fifth removed note", "# sixth removed note"} {
					if strings.Count(string(data), note) != 1 {
						t.Fatalf("comment not retained once: %s\n%s", note, data)
					}
					pos := strings.Index(string(data), note)
					if pos <= last {
						t.Fatalf("comment order changed: %s", data)
					}
					last = pos
				}
				if !strings.HasSuffix(string(data), suffix) {
					t.Fatalf("outside comments changed: %s", data)
				}
				if previous != nil && string(previous) != string(data) {
					t.Fatalf("repeated save unstable:\n%s\n%s", previous, data)
				}
				previous = data
				entry = lib.Entries[0]
				lib, _, err = lib.Save(entry, entry.Snippet, "")
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRemovedCommentsDoNotDuplicateMatchingSurvivorText(t *testing.T) {
	path := fixture(t, "config.yaml", "snippets:\n  - name: Original # shared note\n    command: echo safe\n    inputs:\n      - name: keep\n      - name: gone\n        help: old # first removed note\n        special:\n          old: output # shared note\n")
	lib := Load(path, t.TempDir())
	if err := lib.Error(); err != nil {
		t.Fatal(err)
	}
	entry := lib.Entries[0]
	draft := entry.Snippet
	draft.Inputs = draft.Inputs[:1]
	lib, _, err := lib.Save(entry, draft, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(data), "# shared note") != 2 || strings.LastIndex(string(data), "# shared note") < strings.Index(string(data), "# first removed note") {
			t.Fatalf("comment occurrences/order changed: %s", data)
		}
		entry = lib.Entries[0]
		lib, _, err = lib.Save(entry, entry.Snippet, "")
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestFinalFieldPreservesExplicitEndMarker(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, operation := range []string{"add", "settings"} {
			t.Run(operation+"/"+map[bool]string{true: "CRLF", false: "LF"}[newline == "\r\n"], func(t *testing.T) {
				body := "snippets: []\n"
				if operation == "settings" {
					body += "settings: {color: auto}\n"
				}
				suffix := "\n# before end\n... # explicit end\n# trailing comment\n\n"
				path := fixture(t, "config.yaml", strings.ReplaceAll(body+suffix, "\n", newline))
				lib := Load(path, t.TempDir())
				if err := lib.Error(); err != nil {
					t.Fatal(err)
				}
				if operation == "add" {
					if _, _, err := lib.Save(nil, models.Snippet{Name: "Added", Command: "echo safe"}, path); err != nil {
						t.Fatal(err)
					}
				} else {
					settings := lib.Settings
					settings.Color = "never"
					if _, err := lib.SaveSettings(settings); err != nil {
						t.Fatal(err)
					}
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.HasSuffix(string(data), strings.ReplaceAll(suffix, "\n", newline)) {
					t.Fatalf("end-marker suffix changed: %q", data)
				}
			})
		}
	}
}
