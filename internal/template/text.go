package template

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Text edits rune positions, never byte offsets. Values remain literal; only
// the terminal-facing view escapes control characters.
type Text struct {
	Value     []rune
	Cursor    int
	Multiline bool
	Mark      int
}

func NewText(value string, multiline bool) Text {
	r := []rune(value)
	return Text{Value: r, Cursor: len(r), Multiline: multiline, Mark: -1}
}
func (t Text) String() string    { return string(t.Value) }
func (t *Text) Set(value string) { t.Value = []rune(value); t.Cursor = min(t.Cursor, len(t.Value)) }
func (t *Text) Insert(value []rune) {
	t.Cursor = max(0, min(t.Cursor, len(t.Value)))
	tail := append([]rune{}, t.Value[t.Cursor:]...)
	t.Value = append(t.Value[:t.Cursor], value...)
	t.Value = append(t.Value, tail...)
	t.Cursor += len(value)
}
func (t *Text) Update(key tea.KeyMsg) bool {
	t.Cursor = max(0, min(t.Cursor, len(t.Value)))
	before := t.String()
	switch key.String() {
	case "left":
		t.Cursor = max(0, t.Cursor-1)
	case "right":
		t.Cursor = min(len(t.Value), t.Cursor+1)
	case "home", "ctrl+a":
		for t.Cursor > 0 && t.Value[t.Cursor-1] != '\n' {
			t.Cursor--
		}
	case "end", "ctrl+e":
		for t.Cursor < len(t.Value) && t.Value[t.Cursor] != '\n' {
			t.Cursor++
		}
	case "backspace", "ctrl+h":
		if t.Cursor > 0 {
			t.Value = append(t.Value[:t.Cursor-1], t.Value[t.Cursor:]...)
			t.Cursor--
		}
	case "delete":
		if t.Cursor < len(t.Value) {
			t.Value = append(t.Value[:t.Cursor], t.Value[t.Cursor+1:]...)
		}
	case "ctrl+x":
		t.Value = nil
		t.Cursor = 0
	case "ctrl+w":
		end := t.Cursor
		for t.Cursor > 0 && unicode.IsSpace(t.Value[t.Cursor-1]) {
			t.Cursor--
		}
		for t.Cursor > 0 && !unicode.IsSpace(t.Value[t.Cursor-1]) {
			t.Cursor--
		}
		t.Value = append(t.Value[:t.Cursor], t.Value[end:]...)
	case "ctrl+y":
		end := t.Cursor
		for end < len(t.Value) && t.Value[end] != '\n' {
			end++
		}
		t.Value = append(t.Value[:t.Cursor], t.Value[end:]...)
	case "alt+enter":
		if t.Multiline {
			t.Insert([]rune{'\n'})
		}
	case "up", "down":
		if t.Multiline {
			column := 0
			start := t.Cursor
			for start > 0 && t.Value[start-1] != '\n' {
				start--
				column++
			}
			if key.String() == "up" && start > 0 {
				end := start - 1
				start = end
				for start > 0 && t.Value[start-1] != '\n' {
					start--
				}
				t.Cursor = min(start+column, end)
			} else if key.String() == "down" {
				end := t.Cursor
				for end < len(t.Value) && t.Value[end] != '\n' {
					end++
				}
				if end < len(t.Value) {
					start = end + 1
					end = start
					for end < len(t.Value) && t.Value[end] != '\n' {
						end++
					}
					t.Cursor = min(start+column, end)
				}
			}
		}
	default:
		if key.Type == tea.KeyRunes {
			runes := key.Runes
			if !t.Multiline {
				runes = []rune(strings.ReplaceAll(strings.ReplaceAll(string(runes), "\n", " "), "\r", " "))
			}
			t.Insert(runes)
		}
	}
	if before != t.String() {
		t.Mark = -1
		return true
	}
	return false
}
func Safe(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch {
		case r == '\n':
			out.WriteRune(r)
		case r == '\t':
			out.WriteString("\\t")
		case unicode.IsControl(r):
			switch r {
			case '\r':
				out.WriteString("\\r")
			case '\x1b':
				out.WriteString("\\x1b")
			default:
				out.WriteString("\\u")
				const digits = "0123456789abcdef"
				for _, shift := range []int{12, 8, 4, 0} {
					out.WriteByte(digits[(r>>shift)&15])
				}
			}
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
func (t Text) OneLine(width int, focused bool) string {
	if width <= 0 {
		return ""
	}
	position := max(0, min(t.Cursor, len(t.Value)))
	left := 0
	if focused {
		left = max(0, ansi.StringWidth(Safe(string(t.Value[:position])))-width+1)
	}
	return ansi.Truncate(ansi.TruncateLeft(t.View(focused), left, ""), width, "")
}
func (t Text) View(focused bool) string {
	position := max(0, min(t.Cursor, len(t.Value)))
	if !focused {
		return Safe(t.String())
	}
	return Safe(string(t.Value[:position])) + "|" + Safe(string(t.Value[position:]))
}
func Fit(value string, width, height, offset int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(ansi.Wrap(value, width, ""), "\n")
	offset = max(0, min(offset, max(0, len(lines)-height)))
	end := min(len(lines), offset+height)
	return strings.Join(lines[offset:end], "\n")
}
