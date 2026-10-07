package template

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

type Control struct {
	ID, Label, Help, Error string
	// Missing marks an empty required value. It is shown once, quietly, as a
	// "required" placeholder rather than as an error.
	Missing bool
	// Summary replaces "[Enter]" as an action row's aligned value.
	Summary string
	// LabelOnly navigation rows (Back, Add) span the row and do not widen the
	// aligned label column.
	LabelOnly bool
	// Note rows are read-only explanations (e.g. what a computed value uses and
	// yields). Navigation skips them and they never take focus.
	Note bool
	// Rule renders a Note as a full-width separator line (e.g. between repeated groups).
	Rule bool
	// Placeholder is a dim example shown in an empty text field (never saved).
	Placeholder string
	// OptionLabels optionally shows friendlier text for Options (same order);
	// the stored value is still the option itself.
	OptionLabels []string
	Text         *Text
	Selection    *string
	Options      []string
	Toggle       *bool
	Action       func()
	Submit       bool
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
func (c Control) label() string {
	label := Safe(c.Label)
	if c.Help != "" {
		label += " (" + Safe(c.Help) + ")"
	}
	return label
}

func (c Control) View(focused bool) string {
	return c.view(focused, mutedControlPalette)
}

func (c Control) view(focused bool, palette controlPalette) string {
	r := palette.Renderer
	if r == nil {
		r = lipgloss.DefaultRenderer()
	}
	prefix := "  "
	controlLabel := c.label()
	if palette.Compact {
		controlLabel = Safe(c.Label)
	}
	label := palette.Label.Render(controlLabel + ":")
	if focused {
		prefix = palette.Focused.Render("> ")
		label = palette.Focused.Render(controlLabel + ":")
	}
	value := ""
	switch {
	case c.Text != nil:
		value = c.Text.view(focused, r)
	case c.Toggle != nil:
		if *c.Toggle {
			value = "[on]"
		} else {
			value = "[off]"
		}
	case c.Selection != nil:
		items := []string{}
		for i, item := range c.Options {
			label := c.optionLabel(i)
			if item == *c.Selection {
				label = palette.Selected.Render("<" + label + ">")
			} else {
				label = palette.Unselected.Render(" " + label + " ")
			}
			items = append(items, label)
		}
		value = strings.Join(items, " ")
	case c.Action != nil || c.Submit:
		value = "[Enter]"
	}
	line := prefix + label + " " + palette.Value.Render(value)
	if palette.Compact && focused && c.Help != "" {
		line += "\n    " + palette.Label.Render(Safe(c.Help))
	}
	if c.Error != "" {
		line += "\n    " + palette.Error.Render("[Error: "+Safe(c.Error)+"]")
	}
	return line
}
func ControlsView(controls []Control, focus, width, height int) string {
	return controlsView(controls, focus, width, height, mutedControlPalette)
}

func controlsView(controls []Control, focus, width, height int, palette controlPalette) string {
	// Scroll by wrapped lines so a long focused editor remains reachable.
	lines := []string{}
	focusLine := 0
	for i, control := range controls {
		content := Fit(control.view(i == focus, palette), width, 100000, 0)
		if i == focus {
			focusLine = len(lines) + control.focusRow(content, palette)
		}
		if i == focus && palette.HighlightRow {
			content = FillRow(palette.Row, content, width)
		}
		lines = append(lines, strings.Split(content, "\n")...)
	}
	context := min(max(0, height-1), max(1, height/3))
	if palette.Compact && height <= 2 {
		context = 0
	}
	offset := max(0, focusLine-context)
	return Fit(strings.Join(lines, "\n"), width, height, offset)
}

// focusRow locates the cursor/selected value in the complete wrapped rendering.
// Wrapping only the text prefix misses words moved by their trailing suffix.
func (c Control) focusRow(content string, palette controlPalette) int {
	label := c.label()
	if palette.Compact {
		label = Safe(c.Label)
	}
	prefix := "> " + label + ":"
	needle, occurrence := "", 1
	switch {
	case c.Text != nil:
		r := palette.Renderer
		if r == nil {
			r = lipgloss.DefaultRenderer()
		}
		if strings.Contains(c.Text.view(true, r), "\x1b[7m") {
			needle = "\x1b[7m"
		} else {
			needle = "|"
			position := max(0, min(c.Text.Cursor, len(c.Text.Value)))
			occurrence += strings.Count(prefix+Safe(string(c.Text.Value[:position])), needle)
		}
	default:
		return 0
	}
	for row, line := range strings.Split(content, "\n") {
		if needle != "\x1b[7m" {
			line = ansi.Strip(line)
		}
		occurrence -= strings.Count(line, needle)
		if occurrence <= 0 {
			return row
		}
	}
	return 0
}

// alignedView renders one row per control with values in a shared column and
// the focused control's label, help and error in a hint pinned to the bottom.
// It also returns the hint's whitespace-normalized plain text.
func alignedView(controls []Control, focus, width, height int, palette controlPalette) (string, string) {
	if width <= 0 || height <= 0 {
		return "", ""
	}
	r := palette.Renderer
	if r == nil {
		r = lipgloss.DefaultRenderer()
	}
	hint, hintText := "", ""
	if focus >= 0 && focus < len(controls) && height >= 3 {
		hint = Fit(controls[focus].hint(palette), width, min(3, height-2), 0)
		hintText = strings.Join(strings.Fields(ansi.Strip(hint)), " ")
		hint = palette.Label.Render(strings.Repeat("─", width)) + "\n" + hint
	}
	rows := height
	if hint != "" {
		rows -= strings.Count(hint, "\n") + 1
	}
	labelWidth := 0
	for _, c := range controls {
		if !c.LabelOnly {
			labelWidth = max(labelWidth, ansi.StringWidth(Safe(c.Label)))
		}
	}
	labelWidth = min(labelWidth, max(4, (width-4)/2))
	lines := []string{}
	focusLine := 0
	// Wells breathe: a blank row between fields whenever everything still fits.
	spaced := palette.Wells && len(controls)*2-1 <= rows
	for i, c := range controls {
		if spaced && i > 0 {
			lines = append(lines, "")
		}
		content, row := c.alignedRow(i == focus, width, labelWidth, palette, r)
		if i == focus {
			focusLine = len(lines) + row
			if palette.HighlightRow {
				content = FillRow(palette.Row, content, width)
			}
		}
		lines = append(lines, strings.Split(content, "\n")...)
	}
	context := min(max(0, rows-1), max(1, rows/3))
	offset := max(0, min(focusLine-context, len(lines)-rows))
	visible := append([]string{}, lines[offset:min(len(lines), offset+rows)]...)
	if hint != "" {
		for len(visible) < rows {
			visible = append(visible, "")
		}
		visible = append(visible, hint)
	}
	return strings.Join(visible, "\n"), hintText
}

// hint explains the focused field: its description (or label when it has
// none) and any real validation error. A missing required value is already
// marked in place, so it is not repeated here.
func (c Control) hint(palette controlPalette) string {
	parts := []string{}
	if c.Help != "" {
		parts = append(parts, palette.Label.Render(Safe(c.Help)))
	} else {
		parts = append(parts, palette.Focused.Render(Safe(c.Label)))
	}
	if c.Error != "" {
		parts = append(parts, palette.Error.Render("! "+Safe(c.Error)))
	}
	return strings.Join(parts, palette.Label.Render(" · "))
}

// alignedRow returns the control's row(s) and the row holding its cursor.
func (c Control) alignedRow(focused bool, width, labelWidth int, palette controlPalette, r *lipgloss.Renderer) (string, int) {
	prefix, labelStyle := "  ", palette.Label
	if focused {
		prefix, labelStyle = palette.Focused.Render("> "), palette.Focused
		if palette.Wells {
			prefix = palette.Accent.Render("▌ ")
		}
	}
	label := Safe(c.Label)
	if c.Rule {
		text := ansi.Truncate(label, max(1, width-2), "…")
		return palette.Label.Render(text + strings.Repeat("─", max(0, width-ansi.StringWidth(text)))), 0
	}
	if c.LabelOnly {
		return prefix + labelStyle.Render(ansi.Truncate(label, max(1, width-2), "…")), 0
	}
	label = ansi.Truncate(label, labelWidth, "…")
	lead := prefix + labelStyle.Render(label) + strings.Repeat(" ", labelWidth-ansi.StringWidth(label)+2)
	mark := ""
	if c.Error != "" && !focused {
		mark = " " + palette.Error.Render("!")
	}
	valueWidth := max(1, width-labelWidth-4-ansi.StringWidth(mark))
	// A well is a filled input box: one cell of padding, then the value.
	well := func(value string) string { return value }
	if palette.Wells && (c.Text != nil || c.Selection != nil || c.Toggle != nil) {
		valueWidth = max(1, valueWidth-1)
		style := palette.Well
		if focused {
			style = palette.FocusWell
		}
		well = func(value string) string { return FillRow(style, " "+value, valueWidth+1) }
	}
	value := ""
	switch {
	case c.Text != nil && !focused && len(c.Text.Value) == 0:
		switch {
		case c.Missing:
			value = palette.Label.Render("required")
		case c.Placeholder != "":
			value = palette.Label.Render(ansi.Truncate(Safe(c.Placeholder), valueWidth, "…"))
		case !palette.Wells:
			value = palette.Label.Render("—")
		}
	case c.Text != nil && c.Text.Multiline:
		wrapped := strings.Split(ansi.Wrap(c.Text.view(focused, r), valueWidth, ""), "\n")
		row := 0
		if focused {
			row = cursorRow(wrapped, *c.Text, r)
		}
		for i := range wrapped {
			wrapped[i] = well(palette.Value.Render(wrapped[i]))
		}
		return lead + strings.Join(wrapped, "\n"+strings.Repeat(" ", labelWidth+4)) + mark, row
	case c.Text != nil && focused && c.Missing && len(c.Text.Value) == 0 && valueWidth > 10:
		value = requiredPlaceholder(palette.Label, r)
	case c.Text != nil && focused && c.Placeholder != "" && len(c.Text.Value) == 0 && valueWidth > 10:
		value = palette.Value.Render(textWindow(*c.Text, 1, r)) + palette.Label.Render(ansi.Truncate(Safe(c.Placeholder), valueWidth-1, "…"))
	case c.Text != nil && focused:
		value = palette.Value.Render(textWindow(*c.Text, valueWidth, r))
	case c.Text != nil:
		value = palette.Value.Render(ansi.Truncate(strings.ReplaceAll(Safe(c.Text.String()), "\n", " "), valueWidth, "…"))
	case c.Toggle != nil:
		value = "[off]"
		if *c.Toggle {
			value = "[on]"
		}
		value = palette.Value.Render(value)
	case c.Selection != nil:
		value = palette.Value.Render(c.choiceWindow(valueWidth, palette))
	case c.Action != nil || c.Submit:
		value = "[Enter]"
		if c.Summary != "" {
			value = Safe(c.Summary)
		}
		value = palette.Value.Render(ansi.Truncate(value, valueWidth, "…"))
	}
	return lead + well(value) + mark, 0
}

// requiredPlaceholder shows an empty required field's quiet marker with the
// cursor resting on its first letter, so the value column stays aligned.
// Plain terminals keep the visible "|" cursor before it.
func requiredPlaceholder(dim lipgloss.Style, r *lipgloss.Renderer) string {
	if r.ColorProfile() == termenv.Ascii {
		return "|" + dim.Render("required")
	}
	return r.NewStyle().Reverse(true).Render("r") + dim.Render("equired")
}

// textWindow scrolls a focused single-line value horizontally, keeping the
// cursor and some following text visible.
func textWindow(t Text, width int, r *lipgloss.Renderer) string {
	glyph := ansi.StringWidth(t.cursorGlyphWithRenderer(r))
	if glyph > width {
		return r.NewStyle().Reverse(true).Render(" ")
	}
	view := strings.ReplaceAll(t.view(true, r), "\n", " ")
	position := max(0, min(t.Cursor, len(t.Value)))
	before := ansi.StringWidth(Safe(string(t.Value[:position])))
	after := ansi.StringWidth(view) - before - glyph
	left := max(0, before+glyph+min(after, width/4)-width)
	return ansi.Truncate(ansi.TruncateLeft(view, left, ""), width, "")
}

// optionLabel is the display text for Options[i].
func (c Control) optionLabel(i int) string {
	if i < len(c.OptionLabels) && c.OptionLabels[i] != "" {
		return Safe(c.OptionLabels[i])
	}
	if c.Options[i] == "" {
		return "(empty)"
	}
	return Safe(c.Options[i])
}

// choiceWindow keeps the selected option visible when options overflow.
func (c Control) choiceWindow(width int, palette controlPalette) string {
	items := []string{}
	selectedEnd, offset := 0, 0
	for i, option := range c.Options {
		label := c.optionLabel(i)
		if option == *c.Selection {
			label = palette.Selected.Render("<" + label + ">")
			selectedEnd = offset + ansi.StringWidth(label)
		} else {
			label = palette.Unselected.Render(" " + label + " ")
		}
		offset += ansi.StringWidth(label) + 1
		items = append(items, label)
	}
	joined := strings.Join(items, " ")
	left := max(0, selectedEnd-width)
	return ansi.Truncate(ansi.TruncateLeft(joined, left, ""), width, "")
}

func cursorRow(lines []string, t Text, r *lipgloss.Renderer) int {
	needle, occurrence := "\x1b[7m", 1
	if !strings.Contains(strings.Join(lines, ""), needle) {
		needle = "|"
		occurrence += strings.Count(Safe(string(t.Value[:max(0, min(t.Cursor, len(t.Value)))])), needle)
	}
	for row, line := range lines {
		if needle == "|" {
			line = ansi.Strip(line)
		}
		occurrence -= strings.Count(line, needle)
		if occurrence <= 0 {
			return row
		}
	}
	return 0
}
