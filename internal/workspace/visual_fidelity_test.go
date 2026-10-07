package workspace

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestDetailSourceRemainsVisibleWithLongPreviewAndInputs(t *testing.T) {
	m, err := New(fixtureLibrary(t, "snippets:\n  - name: Inputs\n    command: echo {{a}} {{b}}\n    inputs:\n      - {name: a, help: First input with a lengthy description}\n      - {name: b, help: Second input with a lengthy description}\n"), Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height, m.Focus = 90, 24, 3
	m.Library.Entries[0].Source.DisplayPath = "public.yaml"
	view := m.View()
	if !strings.Contains(view, "Source: public.yaml") {
		t.Fatalf("detail source clipped: %s", view)
	}
}

func TestLibraryFramedPanesAndCommandTagMetadata(t *testing.T) {
	m, err := New(fixtureLibrary(t, "snippets:\n  - name: Pod usage\n    description: Show CPU usage\n    tags: [Kubernetes, Monitoring]\n    command: kubectl top pods\n"), Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Width, m.Height = 120, 30
	view := m.View()
	for _, want := range []string{"╭", "╰", "Categories", "Commands (1)", "Preview", "Kubernetes · Monitoring", "Command Preview", "Source:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q: %s", want, view)
		}
	}
	for _, size := range [][2]int{{120, 30}, {90, 24}, {60, 18}, {40, 10}, {24, 6}, {0, 0}} {
		m.Width, m.Height = size[0], size[1]
		view = m.View()
		if size[1] > 0 && strings.Count(view, "\n")+1 > size[1] {
			t.Fatalf("height exceeded %v", size)
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("width exceeded %v: %s", size, line)
			}
		}
	}
}
