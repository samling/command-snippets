package workspace

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRecoveryKeepsActionableShortcutsAtSmallSizes(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		settings     bool
	}{
		{"malformed", "settings: [\n", false},
		{"missing source", "settings:\n  sources: [missing-source-with-a-long-name.yaml]\nsnippets: []\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := New(fixtureLibrary(t, tc.config), Options{NoColor: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, size := range [][2]int{{24, 6}, {40, 10}, {80, 24}} {
				m.Width, m.Height = size[0], size[1]
				view := m.View()
				for _, shortcut := range []string{"Ctrl-R:", "Esc:"} {
					if !strings.Contains(view, shortcut) {
						t.Fatalf("%s hidden at %v: %q", shortcut, size, view)
					}
				}
				if strings.Contains(view, "Ctrl-O:") != tc.settings || strings.Count(view, "\n")+1 > m.Height {
					t.Fatalf("wrong recovery actions or height at %v: %q", size, view)
				}
				for _, line := range strings.Split(view, "\n") {
					if ansi.StringWidth(line) > m.Width {
						t.Fatalf("recovery width exceeded at %v: %q", size, line)
					}
				}
			}
		})
	}
}
