package template

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type Control struct {
	ID, Label, Help, Error string
	Text                   *Text
	Selection              *string
	Options                []string
	Toggle                 *bool
	Action                 func()
}

func (c Control) Update(key tea.KeyMsg) bool {
	if c.Text != nil {
		return c.Text.Update(key)
	}
	if c.Toggle != nil {
		if key.String() == "left" || key.String() == "right" || key.String() == " " {
			*c.Toggle = !*c.Toggle
			return true
		}
		return false
	}
	if c.Selection != nil && len(c.Options) > 0 {
		direction := 0
		switch key.String() {
		case "left":
			direction = -1
		case "right", " ":
			direction = 1
		}
		if direction != 0 {
			idx := 0
			for i, v := range c.Options {
				if v == *c.Selection {
					idx = i
					break
				}
			}
			idx = (idx + direction + len(c.Options)) % len(c.Options)
			*c.Selection = c.Options[idx]
			return true
		}
	}
	if c.Action != nil && key.String() == "enter" {
		c.Action()
		return true
	}
	return false
}
func (c Control) View(focused bool) string {
	prefix := "  "
	if focused {
		prefix = "> "
	}
	value := ""
	switch {
	case c.Text != nil:
		value = c.Text.View(focused)
	case c.Toggle != nil:
		if *c.Toggle {
			value = "[on]"
		} else {
			value = "[off]"
		}
	case c.Selection != nil:
		items := []string{}
		for _, item := range c.Options {
			label := Safe(item)
			if item == "" {
				label = "(empty)"
			}
			if item == *c.Selection {
				label = "[" + label + "]"
			}
			items = append(items, label)
		}
		value = strings.Join(items, " | ")
	case c.Action != nil:
		value = "[Enter]"
	}
	line := prefix + Safe(c.Label) + ": " + value
	if focused && c.Help != "" {
		line += "\n    " + Safe(c.Help)
	}
	if c.Error != "" {
		line += "\n    ! " + Safe(c.Error)
	}
	return line
}
func ControlsView(controls []Control, focus, width, height int) string {
	// Scroll by wrapped lines so a long focused editor remains reachable.
	lines := []string{}
	focusLine := 0
	for i, control := range controls {
		if i == focus {
			focusLine = len(lines)
			if control.Text != nil {
				position := max(0, min(control.Text.Cursor, len(control.Text.Value)))
				before := "> " + Safe(control.Label) + ": " + Safe(string(control.Text.Value[:position]))
				focusLine += strings.Count(ansi.Wrap(before, width, ""), "\n")
			}
		}
		content := Fit(control.View(i == focus), width, 100000, 0)
		lines = append(lines, strings.Split(content, "\n")...)
	}
	offset := max(0, focusLine-max(1, height/3))
	return Fit(strings.Join(lines, "\n"), width, height, offset)
}
