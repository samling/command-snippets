package workspace

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/samling/command-snippets/internal/models"
	forms "github.com/samling/command-snippets/internal/template"
	"github.com/samling/command-snippets/internal/templating"
)

// IDs of the left-pane actions that open the right pane. Shift-Tab or Up from
// the right pane's first control returns to the action that opened it.
const (
	openInputsID   = "$open-inputs"
	openComputedID = "$open-computed"
	openTestID     = "$open-test"
	addComputedID  = "$add-computed"
	tagsID         = "$tags"
)

func (e *Editor) opener() string {
	switch e.Section {
	case 2:
		return openComputedID
	case 3:
		return openTestID
	}
	return openInputsID
}

// focusID moves focus to the control with id and reports whether it exists.
func (e *Editor) focusID(id string) bool {
	for i, c := range e.Controls {
		if c.ID == id {
			e.Focus = i
			return true
		}
	}
	return false
}

// step moves focus forward or backward, skipping read-only note rows.
func (e *Editor) step(direction int) {
	if len(e.Controls) == 0 {
		return
	}
	// Leaving the right pane backwards returns to the action that opened it.
	if direction < 0 && !e.Picker && e.Focus == e.firstPaneControl() && e.focusID(e.opener()) {
		return
	}
	for range e.Controls {
		e.Focus = (e.Focus + direction + len(e.Controls)) % len(e.Controls)
		if !e.Controls[e.Focus].Note {
			return
		}
	}
}

// firstPaneControl is the first focusable control of the right pane (notes and
// separators are skipped), or -1 when the pane has none.
func (e *Editor) firstPaneControl() int {
	for i := e.BasicsCount; i < len(e.Controls); i++ {
		if !e.Controls[i].Note {
			return i
		}
	}
	return -1
}

// tagItem is one row of the Tags menu: an existing tag, or creating the typed word.
type tagItem struct {
	Label   string
	Count   int
	Checked bool
	Create  bool
}

// tagParts splits the Tags field into chosen tags and the word being typed.
func (e *Editor) tagParts() ([]string, string) {
	value := e.TagsText.String()
	fields := strings.Fields(value)
	if !e.TagTyping || len(fields) == 0 || strings.HasSuffix(value, " ") || strings.HasSuffix(value, "\t") {
		return fields, ""
	}
	return fields[:len(fields)-1], fields[len(fields)-1]
}

func (e *Editor) tagItems() []tagItem {
	chosen, partial := e.tagParts()
	picked := map[string]bool{}
	for _, tag := range chosen {
		picked[models.TagKey(tag)] = true
	}
	needle := models.TagKey(partial)
	items := []tagItem{}
	exact := false
	known := map[string]bool{}
	add := func(label string, count int) {
		key := models.TagKey(label)
		known[key] = true
		if needle != "" && !strings.Contains(key, needle) {
			return
		}
		exact = exact || key == needle
		items = append(items, tagItem{Label: label, Count: count, Checked: picked[key]})
	}
	for _, c := range e.KnownTags {
		add(c.Label, c.Count)
	}
	// Tags chosen here but not yet saved anywhere (just created, or only on
	// this command) still appear, so they can be seen and unticked.
	for _, tag := range chosen {
		if !known[models.TagKey(tag)] {
			add(tag, 0)
		}
	}
	// Prefix matches first, then by popularity; stable for equal counts.
	sort.SliceStable(items, func(i, j int) bool {
		pi, pj := strings.HasPrefix(models.TagKey(items[i].Label), needle), strings.HasPrefix(models.TagKey(items[j].Label), needle)
		if pi != pj {
			return pi
		}
		return items[i].Count > items[j].Count
	})
	if partial != "" && !exact && !picked[needle] {
		items = append(items, tagItem{Label: partial, Create: true})
	}
	return items
}

// tagsFocused reports whether keyboard input currently belongs to the Tags menu.
func (e *Editor) tagsFocused() bool {
	return e.TagEditing && !e.Picker && e.Focus >= 0 && e.Focus < len(e.Controls) && e.Controls[e.Focus].ID == tagsID
}

// tagKey handles Tags-menu keys; it reports whether the key was consumed.
func (e *Editor) tagKey(key string) bool {
	items := e.tagItems()
	e.TagMenu = max(0, min(e.TagMenu, len(items)-1))
	switch key {
	case "up", "down":
		if len(items) == 0 {
			return false
		}
		if key == "up" {
			e.TagMenu = (e.TagMenu + len(items) - 1) % len(items)
		} else {
			e.TagMenu = (e.TagMenu + 1) % len(items)
		}
		return true
	case "enter":
		if len(items) == 0 {
			return false
		}
		e.applyTag(items[e.TagMenu])
		return true
	case "backspace":
		// Backspace only edits the typed filter; it never removes chosen tags
		// (Enter on a ticked tag unticks it).
		if _, partial := e.tagParts(); partial == "" {
			return true
		}
		return false
	}
	return false
}

func (e *Editor) applyTag(item tagItem) {
	chosen, _ := e.tagParts()
	if item.Checked {
		kept := []string{}
		for _, tag := range chosen {
			if models.TagKey(tag) != models.TagKey(item.Label) {
				kept = append(kept, tag)
			}
		}
		e.setTags(kept, "")
	} else {
		e.setTags(append(chosen, item.Label), "")
	}
	// Keep the highlight on the tag just toggled, even when clearing a typed
	// filter changed the list around it.
	for i, other := range e.tagItems() {
		if models.TagKey(other.Label) == models.TagKey(item.Label) {
			e.TagMenu = i
			break
		}
	}
}

func (e *Editor) setTags(tags []string, partial string) {
	value := strings.Join(tags, " ")
	if value != "" {
		value += " "
	}
	e.TagsText.Set(value + partial)
	e.TagsText.Cursor = len(e.TagsText.Value)
	e.TagMenu = 0
	e.TagTyping = partial != ""
}

// beforeTagType starts a new tag word: typed letters are separated from the
// chosen tags instead of being appended to the last one.
func (e *Editor) beforeTagType(msg tea.KeyMsg) {
	if msg.Type != tea.KeyRunes || len(msg.Runes) == 0 {
		return
	}
	if !e.TagTyping {
		value := e.TagsText.String()
		if value != "" && !strings.HasSuffix(value, " ") {
			e.TagsText.Set(value + " ")
		}
		e.TagsText.Cursor = len(e.TagsText.Value)
	}
	e.TagTyping = true
	e.TagMenu = 0
}

// tagMenu renders the filtering checklist shown under a focused Tags field.
func (e *Editor) tagMenu(s *forms.Styles, width, height int) string {
	items := e.tagItems()
	help := s.Label.Render(forms.Fit("Enter: edit tags · Tab: next field · F1: help", width, 1, 0))
	if e.TagEditing {
		help = s.Label.Render(forms.Fit("↑↓ pick · Enter tick/untick", width, 1, 0) + "\n" + forms.Fit("Type to filter or add a new tag · Tab or Esc: done", width, 1, 0))
	}
	if width < 12 {
		return ""
	}
	if len(items) == 0 {
		return s.Label.Render("No tags yet — type one and press Enter.") + "\n\n" + help
	}
	visible := max(1, height-strings.Count(help, "\n")-4)
	selected := max(0, min(e.TagMenu, len(items)-1))
	start := max(0, min(selected-visible/2, len(items)-visible))
	inner := width
	lines := []string{}
	for i := start; i < min(len(items), start+visible); i++ {
		item := items[i]
		row := "[ ] " + forms.Safe(item.Label)
		if item.Checked {
			row = "[x] " + forms.Safe(item.Label)
		}
		count := ""
		switch {
		case item.Create:
			row = "+ create “" + forms.Safe(item.Label) + "”"
		case item.Count == 0:
			count = "new"
		default:
			count = fmt.Sprintf("(%d)", item.Count)
		}
		prefix := "  "
		if i == selected && e.TagEditing {
			prefix = "> "
		}
		row = ansi.Truncate(prefix+row, max(1, inner-ansi.StringWidth(count)-1), "…")
		row += strings.Repeat(" ", max(1, inner-ansi.StringWidth(row)-ansi.StringWidth(count))) + count
		if i == selected && e.TagEditing {
			row = forms.FillRow(s.ActiveRow, row, inner)
		} else if item.Create {
			row = s.Focused.Render(row)
		}
		lines = append(lines, row)
	}
	if start+visible < len(items) {
		lines = append(lines, s.Label.Render(fmt.Sprintf("  … %d more — type to filter", len(items)-start-visible)))
	}
	return strings.Join(lines, "\n") + "\n\n" + help
}

// withTagDisplay shows the Tags value as "a · b" chips unless its checklist is
// being edited; focused-but-not-editing reads like an action row (Enter opens
// the checklist). The draft text itself is untouched.
func (e *Editor) withTagDisplay(controls []forms.Control, focus int) []forms.Control {
	for i, c := range controls {
		if c.ID != tagsID || (i == focus && e.TagEditing) {
			continue
		}
		chips := strings.Join(strings.Fields(e.TagsText.String()), " · ")
		shown := append([]forms.Control(nil), controls...)
		if i == focus {
			if chips == "" {
				chips = "[Enter]"
			}
			shown[i].Text, shown[i].Summary = nil, chips
		} else if len(strings.Fields(e.TagsText.String())) > 1 {
			text := forms.NewText(chips, false)
			shown[i].Text = &text
		}
		return shown
	}
	return controls
}

// computedName returns a fresh computed-value name that collides with nothing.
func (e *Editor) computedName() string {
	used := map[string]bool{}
	for _, in := range e.Inputs {
		used[in.Name.String()] = true
	}
	for _, p := range e.Expressions {
		used[p.Key.String()] = true
	}
	for n := 1; ; n++ {
		if name := fmt.Sprintf("value_%d", n); !used[name] {
			return name
		}
	}
}

// addComputed appends a computed value with an empty formula and focuses it.
// The formula field shows a dim example instead of starter text, so it is clear
// the formula is the whole expression (not something typed inside quotes).
func (e *Editor) addComputed() {
	e.Expressions = append(e.Expressions, pairDraft{text(e.computedName()), text("")})
	e.PendingFocus = fmt.Sprintf("pairs:%p:%d:value", &e.Expressions, len(e.Expressions)-1)
}

// formulaExample is the dim placeholder for an empty formula.
func (e *Editor) formulaExample() string {
	for _, in := range e.Inputs {
		if name := in.Name.String(); models.ValidIdentifier(name) {
			return "e.g. quote(inputs." + name + ")"
		}
	}
	return `e.g. quote("some text")`
}

var errEmptyFormula = errors.New("write a formula (F1 for examples)")

// nameProblems checks every input and computed value name: required, a valid
// identifier, and unique across both (they share the {{name}} namespace).
func (e *Editor) nameProblems() map[*forms.Text]string {
	type owner struct {
		text *forms.Text
		kind string
	}
	byName := map[string][]owner{}
	order := []owner{}
	for _, in := range e.Inputs {
		o := owner{&in.Name, "input"}
		byName[in.Name.String()] = append(byName[in.Name.String()], o)
		order = append(order, o)
	}
	for i := range e.Expressions {
		p := &e.Expressions[i]
		o := owner{&p.Key, "computed value"}
		byName[p.Key.String()] = append(byName[p.Key.String()], o)
		order = append(order, o)
	}
	article := map[string]string{"input": "an input", "computed value": "a computed value"}
	problems := map[*forms.Text]string{}
	for _, o := range order {
		name := o.text.String()
		switch {
		case strings.TrimSpace(name) == "":
			problems[o.text] = "needs a name"
		case !models.ValidIdentifier(name):
			problems[o.text] = "use letters, digits and _ (not starting with a digit)"
		case len(byName[name]) > 1:
			// Same-kind clashes mark every copy; an input/computed-value clash is
			// the computed value's to rename (inputs are often inferred).
			sameKind, otherKind := false, ""
			for _, other := range byName[name] {
				if other.text == o.text {
					continue
				}
				if other.kind == o.kind {
					sameKind = true
				} else {
					otherKind = other.kind
				}
			}
			switch {
			case sameKind:
				problems[o.text] = "name already used by another " + o.kind
			case o.kind == "computed value" && otherKind != "":
				problems[o.text] = "name already used by " + article[otherKind]
			}
		}
	}
	return problems
}

// firstNameProblem reports the first naming problem in display order.
func (e *Editor) firstNameProblem() error {
	problems := e.nameProblems()
	describe := func(kind, name string) string {
		if strings.TrimSpace(name) == "" {
			return "unnamed " + kind
		}
		return kind + " " + name
	}
	for _, in := range e.Inputs {
		if problem := problems[&in.Name]; problem != "" {
			return fmt.Errorf("%s: %s", describe("input", in.Name.String()), problem)
		}
	}
	for i := range e.Expressions {
		if problem := problems[&e.Expressions[i].Key]; problem != "" {
			return fmt.Errorf("%s: %s", describe("computed value", e.Expressions[i].Key.String()), problem)
		}
	}
	return nil
}

// markNameProblems puts naming problems on the Name fields and input list rows.
func (e *Editor) markNameProblems() {
	problems := e.nameProblems()
	if len(problems) == 0 {
		return
	}
	for i, c := range e.Controls {
		if c.Text != nil && problems[c.Text] != "" {
			e.Controls[i].Error = problems[c.Text]
		}
		for _, in := range e.Inputs {
			if c.ID == inputRowID(in) && problems[&in.Name] != "" {
				e.Controls[i].Error = problems[&in.Name]
			}
		}
	}
}

// computedNotes describes, per computed value row, what it uses and currently
// yields from the Test preview values. Rows are keyed by index so values that
// (temporarily) share a name never share a note.
func (e *Editor) computedNotes() map[int]string {
	notes := map[int]string{}
	for i, p := range e.Expressions {
		if strings.TrimSpace(p.Value.String()) == "" {
			notes[i] = "write a formula above · F1 for examples"
		}
	}
	if e.firstNameProblem() != nil {
		return notes // names must be fixed before values can be evaluated
	}
	saved := e.Expressions
	filled := []pairDraft{}
	for i, p := range saved {
		if notes[i] == "" {
			filled = append(filled, p)
		}
	}
	e.Expressions = filled
	draft, err := e.Draft()
	e.Expressions = saved
	if err != nil {
		return notes
	}
	compiled, err := templating.Compile(draft)
	if err != nil {
		return notes
	}
	result := compiled.Preview(e.validTestValues())
	inCommand := map[string]string{}
	for _, part := range result.Preview {
		if part.Name != "" && !part.Unfilled {
			inCommand[part.Name] = part.Text
		} else if part.Name != "" {
			inCommand[part.Name] = "\x00"
		}
	}
	for i, p := range e.Expressions {
		if notes[i] != "" {
			continue
		}
		name := p.Key.String()
		uses := compiled.Uses(name)
		note := "uses nothing"
		if len(uses) > 0 {
			note = "uses " + strings.Join(uses, ", ")
		}
		switch value, ok := inCommand[name]; {
		case !ok:
			note += " · not in Command yet (add {{" + name + "}})"
		case value == "\x00":
			note += " · needs Test preview values"
		default:
			note += " → " + value
		}
		notes[i] = note
	}
	return notes
}

const computedFormulaHelp = "The whole expression that builds this value; its result shows below. F1: formula guide."

const paneHelpTitle = "\x00help"

// helpAllowed reports whether ? opens help rather than typing: on rows that are
// not free text, and on name fields (identifiers cannot contain ?).
func (e *Editor) helpAllowed() bool {
	if e.Picker || e.Focus < 0 || e.Focus >= len(e.Controls) {
		return false
	}
	c := e.Controls[e.Focus]
	switch {
	case c.Text == nil, c.ID == tagsID && !e.TagEditing:
		return true
	case e.Section == 2 && strings.HasSuffix(c.ID, ":key"):
		return true
	case e.OpenInput != nil && c.ID == inputNameID(e.OpenInput):
		return true
	}
	return false
}

func (e *Editor) helpTopic() string {
	switch {
	case e.Focus < len(e.Controls) && e.Controls[e.Focus].ID == tagsID:
		return "Tags"
	case e.Section == 2:
		return "Computed values"
	case e.Section == 3:
		return "Test preview"
	case e.OpenInput != nil:
		return "Input settings"
	}
	return "Inputs"
}

func (e *Editor) helpText() string {
	lines := map[string][]string{
		"Tags": {
			"Tags group commands into the library's categories.",
			"",
			"Press Enter on Tags to edit them in the checklist. Then:",
			"  type       filter your existing tags, or start a new one",
			"  ↑ ↓        move between tags",
			"  Enter      tick or untick the highlighted tag; on + create \"word\" it adds a new tag",
			"  Backspace  edits what you have typed (never removes a chosen tag)",
			"  Tab / Esc  leave the checklist",
			"",
			"Tags are saved as a list; letter case and duplicates are folded.",
		},
		"Computed values": {
			"A computed value builds one piece of the command from your inputs,",
			"when a plain {{input}} is not enough.",
			"",
			" 1. Give it a Name, e.g. awk_script: letters, digits and _, not used by another input or computed value.",
			" 2. Type {{awk_script}} in the Command field (left) where it goes.",
			" 3. Write its Formula: the whole expression, exactly as written. The line under each value shows the inputs it uses and its current result.",
			"",
			"Formulas:",
			"  inputs.pattern            the value of the input named pattern",
			"  \"text\" + inputs.x        join text (text in double quotes)",
			"  a == b   a != b   !a      compare / negate",
			"  a ? b : c                 if a then b else c",
			"",
			"Helpers:",
			"  quote(x)                  shell-quote x as one argument",
			"  flag(\"-n\", x)             -n x, or nothing when x is empty",
			"  boolFlag(\"-v\", on)        -v when a toggle is on",
			"  repeatFlag(\"-e\", list)    -e a -e b for each list item",
			"  join(list, \",\")           join list items",
			"  default(x, \"y\")           x, or y when x is empty",
			"  empty(x)                  true when x is empty / off",
			"",
			"Examples:",
			"  quote(\"/\" + inputs.pattern + \"/ {print $0}\")",
			"  flag(\"-n\", inputs.namespace)",
			"  inputs.all ? \"-A\" : flag(\"-n\", inputs.namespace)",
		},
		"Test preview": {
			"Test preview fills the inputs with sample values and shows the",
			"command they would produce. Nothing is run or inserted, and the",
			"samples are not saved.",
			"",
			"Use it to check computed values and quoting before you save.",
		},
		"Input settings": {
			"Basics:",
			"  Name             used as {{name}} in the Command field; letters, digits and _, unique",
			"  Shown as         what the form shows instead of the name (optional)",
			"  Description      shown while the field is being filled in (optional)",
			"  Type             text: one value · flag: -n value · toggle: an on/off option · choice: pick one · list: several values",
			"  Flag             for flag, toggle and list: the option written before each value, e.g. -n",
			"  Required         the command can't be inserted while it is empty",
			"",
			"Value:",
			"  Choice / Outputs  for choice: what the form offers, and what goes into the command for it",
			"  Starting value   pre-fill the form; then set what it starts as",
			"",
			"Validation:",
			"  Must match       a regular expression non-empty values must match",
			"  Number range     require a whole number between a minimum and a maximum",
			"  Must be regex    the value itself must be a valid regular expression",
			"",
			"Advanced (Enter to open):",
			"  Substitutions    replace one exact value with other output, e.g. all → -A",
			"  Show only when   hide this input unless another input has a given value",
			"  Required when    require it only when another input has a given value",
			"",
			"Esc or ‹ Back to inputs returns to the list.",
		},
		"Inputs": {
			"Inputs are the values you fill in when you use the command.",
			"",
			"Typing {{folder_name}} in the Command field creates a required text",
			"input automatically; using the same name twice reuses it.",
			"",
			"Enter on an input opens its settings (type, default, validation,",
			"conditions). + Add input creates one by hand. Names must be unique",
			"across inputs and computed values.",
		},
	}
	return strings.Join(lines[e.helpTopic()], "\n")
}

// helpRow wraps an indented reference row. A row like "  quote(x)   shell-quote
// x" keeps its two columns: the description wraps under itself, not under the
// code. Rows without a column gap wrap under their indent.
func helpRow(line string, indent, width int) string {
	trimmed := strings.TrimLeft(line, " ")
	col := -1
	if gap := strings.Index(trimmed, "  "); gap > 0 {
		rest := strings.TrimLeft(trimmed[gap:], " ")
		col = indent + len(trimmed) - len(rest)
	}
	if col < 0 || width-col < 16 {
		pad := strings.Repeat(" ", indent)
		wrapped := strings.Split(forms.Fit(trimmed, max(1, width-indent), 10000, 0), "\n")
		for i := range wrapped {
			wrapped[i] = pad + wrapped[i]
		}
		return strings.Join(wrapped, "\n")
	}
	head := line[:col]
	wrapped := strings.Split(forms.Fit(line[col:], width-col, 10000, 0), "\n")
	for i := range wrapped {
		if i == 0 {
			wrapped[i] = head + wrapped[i]
		} else {
			wrapped[i] = strings.Repeat(" ", col) + wrapped[i]
		}
	}
	return strings.Join(wrapped, "\n")
}

// helpView renders the visible window of the pane help, clamping HelpScroll.
// Plain lines are prose: consecutive ones join into paragraphs that reflow to
// the pane width. Indented lines are reference rows and keep their layout,
// wrapping under their own indent.
func (e *Editor) helpView(width, height int) string {
	out := []string{}
	paragraph := ""
	flush := func() {
		if paragraph != "" {
			out = append(out, forms.Fit(paragraph, width, 10000, 0))
			paragraph = ""
		}
	}
	for _, line := range strings.Split(e.helpText(), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		switch {
		case line == "":
			flush()
			out = append(out, "")
		case indent > 0:
			flush()
			out = append(out, helpRow(line, indent, width))
		case strings.HasSuffix(line, ":") || !strings.Contains(line, " ") || len(line) < 20 && paragraph == "":
			flush()
			out = append(out, line)
		default:
			if paragraph != "" {
				paragraph += " "
			}
			paragraph += line
		}
	}
	flush()
	text := strings.Join(out, "\n")
	lines := strings.Count(text, "\n") + 1
	e.HelpScroll = max(0, min(e.HelpScroll, lines-height))
	return forms.Fit(text, width, height, e.HelpScroll)
}

// leavePane moves focus from the right pane back to the action that opened it.
func (e *Editor) leavePane() bool {
	if e.Picker || e.Focus < e.BasicsCount || e.Focus >= len(e.Controls) {
		return false
	}
	return e.focusID(e.opener())
}
