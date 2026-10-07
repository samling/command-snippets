package workspace

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"sort"
	"strconv"
	"strings"

	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/search"
	forms "github.com/samling/command-snippets/internal/template"
	"github.com/samling/command-snippets/internal/templating"
	"gopkg.in/yaml.v3"
)

type pairDraft struct{ Key, Value forms.Text }
type conditionDraft struct {
	Mode               string
	Input, Value, Expr forms.Text
	Boolean            bool
	Items              []forms.Text
}
type inputDraft struct {
	Name, Label, Help, Flag, Default, Pattern, Min, Max forms.Text
	Kind                                                string
	UseDefault, Required, UseRange, Regex               bool
	ToggleDefault                                       bool
	RepeatDefault                                       []forms.Text
	Choices, Special                                    []pairDraft
	Visible, RequiredWhen                               conditionDraft
}
type Editor struct {
	Styles   *forms.Styles
	TagsText forms.Text
	// KnownTags lists the library's tags (with counts) for the Tags menu.
	KnownTags []search.Category
	// TagMenu is the highlighted row of the Tags menu.
	TagMenu int
	// TagTyping is set once the user types a tag word; until then the menu lists
	// every tag rather than filtering by the last chosen one.
	TagTyping bool
	// TagEditing is set by Enter on Tags: only then do keys drive the checklist,
	// so arrows and Tab scroll straight past Tags.
	TagEditing bool
	// PaneHelp replaces the right pane with help for it, toggled by ?.
	PaneHelp bool
	// AdvancedOpen shows an input's substitutions and show/require conditions.
	AdvancedOpen bool
	// HelpScroll is the first visible line of the pane help.
	HelpScroll      int
	OriginalTags    []string
	InitialTagsText string
	Discovered      map[string]bool
	// OpenInput is the input whose details replace the input list; nil shows the list.
	OpenInput *inputDraft
	// PendingFocus names the control ID Build focuses next; paneFocus selects
	// the right pane's first control.
	PendingFocus           string
	BasicsCount, TestStart int
	CompileError           string

	Entry                                   *library.Entry
	Name, Description, Command, Destination forms.Text
	Inputs                                  []*inputDraft
	Expressions                             []pairDraft
	Section, Focus                          int
	Controls                                []forms.Control
	Initial                                 []byte
	InitialDestination                      string
	Error                                   string
	Test                                    *forms.Form
	TestValues                              map[string]any
	MarkStart, MarkEnd                      int
	Picker                                  bool
	PickerName, PickerFlag                  forms.Text
	PickerKind                              string
	PickerQuotes                            bool
}

func text(v string) forms.Text { return forms.NewText(v, false) }
func newCondition(c *models.Condition) conditionDraft {
	d := conditionDraft{Mode: "none", Input: text(""), Value: text(""), Expr: text("")}
	if c == nil {
		return d
	}
	d.Input = text(c.Input)
	d.Expr = text(c.Expr)
	if c.Expr != "" {
		d.Mode = "expr"
		return d
	}
	value := c.Equals
	d.Mode = "equals"
	if value == nil {
		value = c.NotEquals
		d.Mode = "not_equals"
	}
	switch v := value.(type) {
	case string:
		d.Value = text(v)
	case bool:
		d.Boolean = v
	case []string:
		for _, item := range v {
			d.Items = append(d.Items, text(item))
		}
	case []any:
		for _, item := range v {
			d.Items = append(d.Items, text(fmt.Sprint(item)))
		}
	}
	return d
}
func newInput(in models.Input) *inputDraft {
	d := &inputDraft{Name: text(in.Name), Label: text(in.Label), Help: text(in.Help), Flag: text(in.Flag), Default: text(""), Pattern: text(""), Min: text(""), Max: text(""), Kind: in.InputKind(), UseDefault: in.Default != nil, Required: in.Required, Visible: newCondition(in.VisibleWhen), RequiredWhen: newCondition(in.RequiredWhen)}
	switch v := in.DefaultValue().(type) {
	case string:
		d.Default = text(v)
	case bool:
		d.ToggleDefault = v
	case []string:
		for _, item := range v {
			d.RepeatDefault = append(d.RepeatDefault, text(item))
		}
	}
	for _, c := range in.Choices {
		d.Choices = append(d.Choices, pairDraft{text(c.Label), text(c.Value)})
	}
	keys := []string{}
	for key := range in.Special {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		d.Special = append(d.Special, pairDraft{text(key), text(in.Special[key])})
	}
	if v := in.Validate; v != nil {
		d.Pattern = text(v.Pattern)
		d.Regex = v.Regex
		if len(v.Range) == 2 {
			d.UseRange = true
			d.Min = text(strconv.Itoa(v.Range[0]))
			d.Max = text(strconv.Itoa(v.Range[1]))
		}
	}
	return d
}
func NewEditor(entry *library.Entry, destination string) (*Editor, error) {
	snippet := models.Snippet{}
	if entry != nil {
		snippet = entry.Snippet
		destination = entry.Source.DisplayPath
	}
	e := &Editor{Entry: entry, Name: text(snippet.Name), Description: text(snippet.Description), Command: forms.NewText(snippet.Command, true), Destination: text(destination), PickerName: text("input"), PickerFlag: text(""), PickerKind: "text", TestValues: map[string]any{}, Discovered: map[string]bool{}, TagsText: text(strings.Join(snippet.Tags, " ")), OriginalTags: append([]string(nil), snippet.Tags...), InitialTagsText: strings.Join(snippet.Tags, " ")}

	for _, in := range snippet.Inputs {
		e.Inputs = append(e.Inputs, newInput(in))
	}
	names := []string{}
	for name := range snippet.Expressions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		e.Expressions = append(e.Expressions, pairDraft{text(name), text(snippet.Expressions[name])})
	}
	draft, err := e.Draft()
	if err != nil {
		return nil, err
	}
	e.InitialDestination = destination
	e.Initial, err = yaml.Marshal(draft)
	if err != nil {
		return nil, err
	}
	e.Build()
	return e, nil
}
func (e *Editor) condition(d conditionDraft) *models.Condition {
	if d.Mode == "none" {
		return nil
	}
	if d.Mode == "expr" {
		return &models.Condition{Expr: d.Expr.String()}
	}
	var value any = d.Value.String()
	for _, in := range e.Inputs {
		if in.Name.String() == d.Input.String() {
			if in.Kind == "toggle" {
				value = d.Boolean
			}
			if in.Kind == "repeat" {
				items := []string{}
				for _, item := range d.Items {
					items = append(items, item.String())
				}
				value = items
			}
		}
	}
	c := &models.Condition{Input: d.Input.String()}
	if d.Mode == "equals" {
		c.Equals = value
	} else {
		c.NotEquals = value
	}
	return c
}
func (e *Editor) Draft() (models.Snippet, error) {
	s := models.Snippet{Name: strings.TrimSpace(e.Name.String()), Description: e.Description.String(), Command: e.Command.String()}
	if e.Entry != nil {
		s.ID = e.Entry.Snippet.ID
	}
	if e.TagsText.String() == e.InitialTagsText {
		s.Tags = append([]string(nil), e.OriginalTags...)
	} else {
		seen := map[string]bool{}
		for _, tag := range strings.Fields(e.TagsText.String()) {
			key := models.TagKey(tag)
			if !seen[key] {
				seen[key] = true
				s.Tags = append(s.Tags, tag)
			}
		}
	}
	for _, d := range e.Inputs {
		in := models.Input{Name: d.Name.String(), Label: d.Label.String(), Help: d.Help.String(), Kind: d.Kind, Required: d.Required}
		if d.Kind == "flag" || d.Kind == "toggle" || d.Kind == "repeat" {
			in.Flag = d.Flag.String()
		}
		if in.Kind == "text" {
			in.Kind = ""
		}
		if d.UseDefault {
			switch d.Kind {
			case "toggle":
				in.Default = d.ToggleDefault
			case "repeat":
				items := []string{}
				for _, item := range d.RepeatDefault {
					items = append(items, item.String())
				}
				in.Default = items
			default:
				in.Default = d.Default.String()
			}
		}
		if d.Kind == "choice" {
			for _, p := range d.Choices {
				in.Choices = append(in.Choices, models.Choice{Label: p.Key.String(), Value: p.Value.String()})
			}
		}
		if len(d.Special) > 0 && d.Kind != "toggle" && d.Kind != "repeat" {
			in.Special = map[string]string{}
			for _, p := range d.Special {
				if _, duplicate := in.Special[p.Key.String()]; duplicate {
					return s, fmt.Errorf("%s: duplicate special value", in.Name)
				}
				in.Special[p.Key.String()] = p.Value.String()
			}
		}
		if d.Kind != "toggle" && (d.Pattern.String() != "" || d.UseRange || d.Regex) {
			in.Validate = &models.Validation{Pattern: d.Pattern.String(), Regex: d.Regex}
			if d.UseRange {
				low, err := strconv.Atoi(d.Min.String())
				if err != nil {
					return s, fmt.Errorf("%s: invalid range minimum", in.Name)
				}
				high, err := strconv.Atoi(d.Max.String())
				if err != nil {
					return s, fmt.Errorf("%s: invalid range maximum", in.Name)
				}
				in.Validate.Range = []int{low, high}
			}
		}
		in.VisibleWhen = e.condition(d.Visible)
		in.RequiredWhen = e.condition(d.RequiredWhen)
		s.Inputs = append(s.Inputs, in)
	}
	if err := e.firstNameProblem(); err != nil {
		return s, err
	}
	if len(e.Expressions) > 0 {
		s.Expressions = map[string]string{}
		for _, p := range e.Expressions {
			if strings.TrimSpace(p.Value.String()) == "" {
				return s, fmt.Errorf("computed value %s: %w", p.Key.String(), errEmptyFormula)
			}
			if _, duplicate := s.Expressions[p.Key.String()]; duplicate {
				return s, fmt.Errorf("duplicate expression %s", p.Key.String())
			}
			s.Expressions[p.Key.String()] = p.Value.String()
		}
	}
	return s, nil
}
func (e *Editor) Dirty() bool {
	if e.Entry == nil && e.Destination.String() != e.InitialDestination {
		return true
	}
	draft, err := e.Draft()
	if err != nil {
		return true
	}
	data, err := yaml.Marshal(draft)
	return err != nil || string(data) != string(e.Initial)
}
func (e *Editor) addInput() {
	name := "input"
	for number := 1; ; number++ {
		exists := false
		for _, in := range e.Inputs {
			if in.Name.String() == name {
				exists = true
			}
		}
		if !exists {
			break
		}
		name = fmt.Sprintf("input_%d", number)
	}
	in := newInput(models.Input{Name: name})
	e.Inputs = append(e.Inputs, in)
	e.openInput(in)
}

const (
	advancedID     = "$advanced"
	paneFocus      = "$pane"
	addInputID     = "$add-input"
	backToInputsID = "$back-inputs"
)

func inputRowID(in *inputDraft) string  { return fmt.Sprintf("input:%p", in) }
func inputNameID(in *inputDraft) string { return fmt.Sprintf("input:%p:name", in) }

func (e *Editor) openInput(in *inputDraft) {
	e.OpenInput = in
	e.PendingFocus = inputNameID(in)
	// Advanced starts open only when it already holds something.
	e.AdvancedOpen = len(in.Special) > 0 || in.Visible.Mode != "none" || in.RequiredWhen.Mode != "none"
}

// closeInput returns from details to the list, focused on that input's row.
func (e *Editor) closeInput() {
	if e.OpenInput != nil {
		e.PendingFocus = inputRowID(e.OpenInput)
	}
	e.OpenInput = nil
}

// commandLineMove reports whether Up/Down should move the multiline Command
// cursor rather than focus: it has another line in that direction.
func (e *Editor) commandLineMove(key string) bool {
	c := e.Controls[e.Focus]
	if c.Text == nil || !c.Text.Multiline {
		return false
	}
	cursor := max(0, min(c.Text.Cursor, len(c.Text.Value)))
	if key == "up" {
		return strings.ContainsRune(string(c.Text.Value[:cursor]), '\n')
	}
	return strings.ContainsRune(string(c.Text.Value[cursor:]), '\n')
}

// CloseInputDetails handles Esc while focus is inside an input's details.
func (e *Editor) CloseInputDetails() bool {
	if e.Picker || e.OpenInput == nil || (e.Section != 0 && e.Section != 1) || e.Focus < e.BasicsCount {
		return false
	}
	e.closeInput()
	e.Build()
	return true
}

// inputSummary describes a list row: behavior, its flag and requiredness.
func inputSummary(in *inputDraft) string {
	summary := in.Kind
	if flag := in.Flag.String(); flag != "" && (in.Kind == "flag" || in.Kind == "toggle" || in.Kind == "repeat") {
		summary += " " + flag
	}
	if in.Required {
		summary += " · required"
	}
	return summary
}
func (e *Editor) Build() {
	var focused forms.Control
	if e.Focus >= 0 && e.Focus < len(e.Controls) {
		focused = e.Controls[e.Focus]
	}
	e.inferInputs()
	e.CompileError = ""
	if strings.TrimSpace(e.Command.String()) != "" {
		draft, err := e.Draft()
		if err == nil {
			_, err = templating.Compile(draft)
		}
		if err != nil && !errors.Is(err, errEmptyFormula) {
			e.CompileError = err.Error()
		}
	}
	defer func() {
		if e.PendingFocus != "" {
			for i, c := range e.Controls {
				if c.ID == e.PendingFocus || e.PendingFocus == paneFocus && i >= e.BasicsCount && !c.Note {
					focused, e.Focus = c, i
					break
				}
			}
			e.PendingFocus = ""
		}
		for i, c := range e.Controls {
			if (focused.ID != "" && c.ID == focused.ID) || (focused.ID == "" && ((focused.Text != nil && c.Text == focused.Text) || (focused.Toggle != nil && c.Toggle == focused.Toggle) || (focused.Selection != nil && c.Selection == focused.Selection) || (focused.Action != nil && c.Label == focused.Label))) {
				e.Focus = i
				break
			}
		}
		e.Focus = max(0, min(e.Focus, max(0, len(e.Controls)-1)))
		// Arriving on Tags shows every tag; the stored text is left untouched so
		// unedited legacy multiword tags are preserved.
		if e.Focus < len(e.Controls) && e.Controls[e.Focus].ID == tagsID && focused.ID != tagsID {
			e.TagTyping = false
		}
		if e.Focus >= len(e.Controls) || e.Controls[e.Focus].ID != tagsID {
			e.TagEditing = false
		}
	}()
	e.Controls = nil
	addText := func(label string, t *forms.Text, help string) {
		e.Controls = append(e.Controls, forms.Control{Label: label, Text: t, Help: help})
	}
	addBool := func(label string, b *bool) { e.Controls = append(e.Controls, forms.Control{Label: label, Toggle: b}) }
	addAction := func(label string, action func()) {
		e.Controls = append(e.Controls, forms.Control{Label: label, Action: action})
	}
	if e.Picker {
		addText("Input name", &e.PickerName, "Use a new identifier to create an input, or a declared name to link another occurrence.")
		existing := e.existingInput(e.PickerName.String())
		if existing == nil {
			e.Controls = append(e.Controls, forms.Control{Label: "Behavior", Selection: &e.PickerKind, Options: []string{"text", "flag", "toggle", "choice", "repeat"}})
			if e.PickerKind == "flag" || e.PickerKind == "toggle" || e.PickerKind == "repeat" {
				addText("Flag", &e.PickerFlag, "Literal option, e.g. -n or --name.")
			}
		}
		start, end := e.MarkStart, e.MarkEnd
		outside := start > 0 && end < len(e.Command.Value) && e.Command.Value[start-1] == e.Command.Value[end] && (e.Command.Value[end] == '\'' || e.Command.Value[end] == '"')
		inside := end-start >= 2 && e.Command.Value[start] == e.Command.Value[end-1] && (e.Command.Value[start] == '\'' || e.Command.Value[start] == '"')
		if outside || inside {
			addBool("Treat the whole quoted span as one argument", &e.PickerQuotes)
		}
		label := "Confirm marked input"
		if existing != nil {
			label = "Link existing input " + existing.Name.String()
		}
		addAction(label, e.confirmMark)
		e.Focus = max(0, min(e.Focus, max(0, len(e.Controls)-1)))
		return
	}

	addText("Friendly name", &e.Name, "A searchable display title, not a slug.")
	addText("Command", &e.Command, "Type {{name}} to create an input. Alt-Enter: newline.")
	addText("Description", &e.Description, "")
	tagHelp := "Enter edits tags in the checklist on the right."
	for _, tag := range e.OriginalTags {
		if len(strings.Fields(tag)) > 1 {
			tagHelp += " Existing multiword tags stay intact until Tags is edited; edits split on whitespace."
			break
		}
	}
	addText("Tags", &e.TagsText, tagHelp)
	e.Controls[len(e.Controls)-1].ID = tagsID
	e.Controls[len(e.Controls)-1].Action = func() { e.TagEditing, e.TagTyping = true, false }
	addAction("Configure inputs", func() { e.leaveTest(); e.Section, e.OpenInput, e.PendingFocus = 0, nil, paneFocus })
	e.Controls[len(e.Controls)-1].ID = openInputsID
	addAction("Computed values", func() { e.leaveTest(); e.Section, e.PendingFocus = 2, paneFocus })
	e.Controls[len(e.Controls)-1].ID = openComputedID
	addAction("Test preview", func() { e.leaveTest(); e.Section, e.PendingFocus = 3, paneFocus })
	e.Controls[len(e.Controls)-1].ID = openTestID
	e.BasicsCount = len(e.Controls)
	switch e.Section {
	case 0, 1:
		if e.OpenInput == nil {
			for _, in := range e.Inputs {
				in := in
				name := in.Name.String()
				if name == "" {
					name = "(unnamed)"
				}
				e.Controls = append(e.Controls, forms.Control{ID: inputRowID(in), Label: name, Summary: inputSummary(in), Action: func() { e.openInput(in) }})
			}
			e.Controls = append(e.Controls, forms.Control{ID: addInputID, Label: "+ Add input", LabelOnly: true, Action: e.addInput})
			break
		}
		e.Controls = append(e.Controls, forms.Control{ID: backToInputsID, Label: "‹ Back to inputs", LabelOnly: true, Action: e.closeInput})
		for idx, d := range e.Inputs {
			idx := idx
			if d != e.OpenInput {
				continue
			}
			last := func() *forms.Control { return &e.Controls[len(e.Controls)-1] }
			rule := func(title string) {
				e.Controls = append(e.Controls, forms.Control{Label: "── " + title + " ", LabelOnly: true, Note: true, Rule: true})
			}
			addRow := func(id, label, help string, action func()) {
				e.Controls = append(e.Controls, forms.Control{ID: id, Label: label, LabelOnly: true, Help: help, Action: action})
			}
			pairRows := func(rows *[]pairDraft, left, leftHelp, right, rightHelp, noun string) {
				for i := range *rows {
					i := i
					addText(left, &(*rows)[i].Key, leftHelp)
					last().ID = fmt.Sprintf("pairs:%p:%d:key", rows, i)
					addText(right, &(*rows)[i].Value, rightHelp)
					last().ID = fmt.Sprintf("pairs:%p:%d:value", rows, i)
					addAction("Remove "+noun, func() { *rows = append((*rows)[:i], (*rows)[i+1:]...) })
					last().ID = fmt.Sprintf("pairs:%p:%d:remove", rows, i)
					last().Help = "Delete this " + noun + "."
				}
			}
			addPair := func(rows *[]pairDraft) func() {
				return func() {
					*rows = append(*rows, pairDraft{text(""), text("")})
					e.PendingFocus = fmt.Sprintf("pairs:%p:%d:key", rows, len(*rows)-1)
				}
			}
			itemRows := func(rows *[]forms.Text, label, help, noun string) {
				for i := range *rows {
					i := i
					addText(label, &(*rows)[i], help)
					last().ID = fmt.Sprintf("rows:%p:%d:text", rows, i)
					addAction("Remove "+noun, func() { *rows = append((*rows)[:i], (*rows)[i+1:]...) })
					last().ID = fmt.Sprintf("rows:%p:%d:remove", rows, i)
					last().Help = "Delete this " + noun + "."
				}
				addRow(fmt.Sprintf("rows:%p:add", rows), "+ Add "+noun, "Add another "+noun+".", func() {
					*rows = append(*rows, text(""))
					e.PendingFocus = fmt.Sprintf("rows:%p:%d:text", rows, len(*rows)-1)
				})
			}
			condition := func(label, off, help string, c *conditionDraft) {
				e.Controls = append(e.Controls, forms.Control{Label: label, Selection: &c.Mode, Options: []string{"none", "equals", "not_equals", "expr"}, OptionLabels: []string{off, "equals", "not equals", "formula"}, Help: help})
				switch c.Mode {
				case "none":
					return
				case "expr":
					addText("  formula", &c.Expr, "A true/false formula, e.g. inputs.mode == \"Named\" (F1 on Computed values lists the syntax).")
					return
				}
				addText("  input", &c.Input, "The name of the other input to compare, e.g. mode.")
				kind := "text"
				for _, in := range e.Inputs {
					if in.Name.String() == c.Input.String() {
						kind = in.Kind
					}
				}
				switch kind {
				case "toggle":
					addBool("  is on", &c.Boolean)
					last().Help = "Whether that toggle must be on."
				case "repeat":
					itemRows(&c.Items, "  item", "One value the list must contain.", "compared item")
				default:
					addText("  value", &c.Value, "The exact value typed or chosen there (not the command output).")
				}
			}
			name := d.Name.String()

			rule("Basics")
			addText("Name", &d.Name, "Used as {{"+name+"}} in the Command field; letters, digits and _.")
			last().ID = inputNameID(d)
			addText("Shown as", &d.Label, "What the form shows instead of the name (optional).")
			last().Placeholder = name
			addText("Description", &d.Help, "Shown while the field is being filled in (optional).")
			e.Controls = append(e.Controls, forms.Control{Label: "Type", Selection: &d.Kind, Options: []string{"text", "flag", "toggle", "choice", "repeat"}, OptionLabels: []string{"text", "flag", "toggle", "choice", "list"},
				Help: "text: one value · flag: -n value · toggle: an on/off option · choice: pick one · list: several values."})
			if d.Kind == "flag" || d.Kind == "toggle" || d.Kind == "repeat" {
				addText("Flag", &d.Flag, "The option written before the value, e.g. -n or --name.")
				last().Placeholder = "e.g. -n"
			}
			addBool("Required", &d.Required)
			last().Help = "The command can't be inserted while this is empty."

			rule("Value")
			if d.Kind == "choice" {
				pairRows(&d.Choices, "Choice", "What the form offers, e.g. Memory.", "Outputs", "What goes into the command for it, e.g. 4.", "choice")
				addRow(fmt.Sprintf("pairs:%p:add", &d.Choices), "+ Add choice", "Add an option to pick from.", addPair(&d.Choices))
			}
			addBool("Starting value", &d.UseDefault)
			last().Help = "Pre-fill the form with a value."
			if d.UseDefault {
				switch d.Kind {
				case "toggle":
					addBool("Starts on", &d.ToggleDefault)
					last().Help = "Whether the toggle starts on."
				case "repeat":
					itemRows(&d.RepeatDefault, "Starts with", "One item the list starts with.", "starting item")
				case "choice":
					addText("Starts as", &d.Default, "The choice the form starts on (its label).")
				default:
					addText("Starts as", &d.Default, "The value the form starts with.")
				}
			}

			if d.Kind != "toggle" {
				rule("Validation")
				addText("Must match", &d.Pattern, "A regular expression non-empty values must match, e.g. ^[0-9]+$ (optional).")
				addBool("Number range", &d.UseRange)
				last().Help = "Require a whole number between a minimum and a maximum."
				if d.UseRange {
					addText("Minimum", &d.Min, "The smallest allowed number.")
					addText("Maximum", &d.Max, "The largest allowed number.")
				}
				addBool("Must be regex", &d.Regex)
				last().Help = "The value itself must be a valid regular expression (e.g. a search pattern)."
			}

			arrow := "▸"
			if e.AdvancedOpen {
				arrow = "▾"
			}
			rule("More")
			e.Controls = append(e.Controls, forms.Control{ID: advancedID, Label: "Advanced " + arrow, Summary: "substitutions · show/require when", Help: "Substitutions and conditions to show or require this input; rarely needed.", Action: func() { e.AdvancedOpen = !e.AdvancedOpen }})
			if e.AdvancedOpen {
				if d.Kind == "text" || d.Kind == "flag" || d.Kind == "choice" {
					pairRows(&d.Special, "When typed", "If exactly this is entered…", "Output instead", "…put this in the command instead, e.g. all → -A.", "substitution")
					addRow(fmt.Sprintf("pairs:%p:add", &d.Special), "+ Add substitution", "Replace one specific value with different output, e.g. all → -A.", addPair(&d.Special))
				}
				condition("Show only when", "always", "Hide this input unless another input has a given value.", &d.Visible)
				condition("Required when", "never", "Make it required only when another input has a given value.", &d.RequiredWhen)
			}
			addRow("$remove-input", "Remove this input", "Delete this input (also remove {{"+name+"}} from Command).", func() {
				e.Inputs = append(e.Inputs[:idx], e.Inputs[idx+1:]...)
				e.OpenInput, e.PendingFocus = nil, addInputID
				if idx < len(e.Inputs) {
					e.PendingFocus = inputRowID(e.Inputs[idx])
				}
			})
		}
	case 2:
		notes := e.computedNotes()
		for i := range e.Expressions {
			i := i
			p := &e.Expressions[i]
			heading := p.Key.String()
			if heading == "" {
				heading = "(unnamed)"
			}
			e.Controls = append(e.Controls, forms.Control{Label: "── " + heading + " ", LabelOnly: true, Note: true, Rule: true})
			addText("Name", &p.Key, "Type {{"+heading+"}} in the Command field (left) where this value goes.")
			e.Controls[len(e.Controls)-1].ID = fmt.Sprintf("pairs:%p:%d:key", &e.Expressions, i)
			addText("Formula", &p.Value, computedFormulaHelp)
			e.Controls[len(e.Controls)-1].ID = fmt.Sprintf("pairs:%p:%d:value", &e.Expressions, i)
			e.Controls[len(e.Controls)-1].Placeholder = e.formulaExample()
			if note, ok := notes[i]; ok {
				e.Controls = append(e.Controls, forms.Control{Label: note, LabelOnly: true, Note: true})
			}
			addAction("Remove computed value", func() {
				e.Expressions = append(e.Expressions[:i], e.Expressions[i+1:]...)
				e.PendingFocus = addComputedID
				if i < len(e.Expressions) {
					e.PendingFocus = fmt.Sprintf("pairs:%p:%d:key", &e.Expressions, i)
				}
			})
			e.Controls[len(e.Controls)-1].ID = fmt.Sprintf("pairs:%p:%d:remove", &e.Expressions, i)
		}
		e.Controls = append(e.Controls, forms.Control{ID: addComputedID, Label: "+ Add computed value", LabelOnly: true, Action: e.addComputed})
	case 3:
		if e.Test == nil {
			draft, err := e.Draft()
			if err == nil {
				e.Test, err = forms.NewForm(draft, e.validTestValues())
			}
			if err != nil {
				e.Error = err.Error()
			}

		}
		if e.Test != nil {
			e.TestStart = len(e.Controls)
			e.Controls = append(e.Controls, e.Test.Controls...)
		}
	}
	e.markNameProblems()
	e.Focus = max(0, min(e.Focus, max(0, len(e.Controls)-1)))
}
func (e *Editor) Mark() {
	if e.Command.Mark < 0 {
		e.Command.Mark = e.Command.Cursor
		e.Error = "Start marked. Move to the end, then Ctrl-P again."
		return
	}
	start, end := e.Command.Mark, e.Command.Cursor
	if start > end {
		start, end = end, start
	}
	if start == end {
		e.Error = "Select a nonempty range using the cursor."
		return
	}
	e.MarkStart, e.MarkEnd = start, end
	e.Picker = true
	e.PickerQuotes = false
	e.Command.Mark = -1
	e.Focus = 0
	e.Error = ""
	e.Build()
}
func (e *Editor) confirmMark() {
	name := e.PickerName.String()
	if !models.ValidIdentifier(name) {
		e.Error = "Invalid input name"
		return
	}
	existing := e.existingInput(name)
	for _, p := range e.Expressions {
		if p.Key.String() == name {
			e.Error = "Name belongs to an expression"
			return
		}
	}
	start, end := e.MarkStart, e.MarkEnd
	raw := string(e.Command.Value[start:end])
	if e.PickerQuotes {
		if end-start >= 2 && e.Command.Value[start] == e.Command.Value[end-1] && (e.Command.Value[start] == '\'' || e.Command.Value[start] == '"') {
			raw = string(e.Command.Value[start+1 : end-1])
		} else if start > 0 && end < len(e.Command.Value) && e.Command.Value[start-1] == e.Command.Value[end] && (e.Command.Value[end] == '\'' || e.Command.Value[end] == '"') {
			start--
			end++
		}
	}
	originalOutput := string(e.Command.Value[start:end])
	input := models.Input{Name: name, Kind: e.PickerKind, Default: raw}
	switch e.PickerKind {
	case "flag":
		input.Flag = e.PickerFlag.String()
	case "repeat":
		input.Flag = e.PickerFlag.String()
		input.Default = []string{raw}
	case "toggle":
		input.Flag = e.PickerFlag.String()
		input.Default = true
	case "choice":
		input.Choices = []models.Choice{{Label: "Original", Value: originalOutput}}
		input.Default = "Original"
	}
	if existing == nil {
		if _, err := templating.Compile(models.Snippet{Name: "Marked input", Command: "{{" + name + "}}", Inputs: []models.Input{input}}); err != nil {
			e.Error = err.Error()
			return
		}
		e.Inputs = append(e.Inputs, newInput(input))
	}
	e.Command.Value = append(e.Command.Value[:start], append([]rune("{{"+name+"}}"), e.Command.Value[end:]...)...)
	e.Command.Cursor = start + len([]rune(name)) + 4
	e.Picker = false
	e.Section = 1
	e.Focus = 0
	e.Test = nil
	e.Error = ""
	e.Build()
}
func (e *Editor) existingInput(name string) *inputDraft {
	for _, in := range e.Inputs {
		if in.Name.String() == name {
			return in
		}
	}
	return nil
}
func (e *Editor) validTestValues() map[string]any {
	values := map[string]any{}
	for _, in := range e.Inputs {
		value, exists := e.TestValues[in.Name.String()]
		if !exists {
			continue
		}
		valid := false
		switch in.Kind {
		case "toggle":
			_, valid = value.(bool)
		case "repeat":
			_, valid = value.([]string)
		default:
			_, valid = value.(string)
		}
		if valid {
			values[in.Name.String()] = value
		}
	}
	return values
}
func (e *Editor) styles() *forms.Styles {
	if e.Styles != nil {
		return e.Styles
	}
	return forms.DefaultStyles()
}
func (e *Editor) leaveTest() {
	if e.Test != nil {
		e.TestValues = e.Test.Values()
	}
	e.Test = nil
}
func (e *Editor) inferInputs() {
	if e.Discovered == nil {
		e.Discovered = map[string]bool{}
	}
	for _, in := range e.Inputs {
		e.Discovered[in.Name.String()] = true
	}
	for _, p := range e.Expressions {
		e.Discovered[p.Key.String()] = true
	}
	for _, name := range templating.Placeholders(e.Command.String()) {
		if !e.Discovered[name] {
			e.Inputs = append(e.Inputs, newInput(models.Input{Name: name, Required: true}))
			e.Discovered[name] = true
		}
	}
	open := false
	for _, in := range e.Inputs {
		open = open || in == e.OpenInput
	}
	if !open {
		e.OpenInput = nil
	}
}
func (e *Editor) testContent(controls []forms.Control, focus, width, height int) string {
	if height <= 0 {
		return ""
	}
	s := e.styles()
	e.Test.Styles = s
	previewRows := min(max(1, height/3), max(1, height-1))
	preview := forms.Fit(s.Preview.Render("Preview: ")+e.Test.CommandPreview(), width, previewRows, 0)
	// Unfocused Test rows only show an error mark; name them (the focused
	// row's reason is already in its hint).
	skip := -1
	if e.Focus >= e.TestStart {
		skip = e.Focus - e.TestStart
	}
	if errs := e.Test.FieldErrors(skip); errs != "" && height-previewRows > 2 {
		preview += "\n" + s.Error.Render(forms.Fit(errs, width, min(2, height-previewRows-2), 0))
	}
	used := strings.Count(preview, "\n") + 1
	if used >= height {
		return preview
	}
	return preview + "\n" + s.AlignedControlsView(controls, focus, width, height-used)
}

func (e *Editor) View(width, height int) string {
	s := e.styles()
	header := "CS — Edit command"
	if e.Dirty() {
		header += " *"
	}
	footer := forms.Fit("Ctrl-S:save Esc:back\nTab/↑↓:control Enter:open F1:help", width, min(2, height), 0)
	footerRows := strings.Count(footer, "\n") + 1
	if height < 10 {
		footer = forms.Fit("Ctrl-S:save Esc:back", width, 1, 0)
		footerRows = 1
	}
	errorRows := 0
	errorLine := ""
	diagnostic := e.Error
	if diagnostic == "" {
		diagnostic = e.CompileError
	}
	if e.Section == 3 && e.Test != nil && e.Test.NonfieldErrors() != "" {
		diagnostic = e.Test.NonfieldErrors()
	}
	headerRows := 2
	if height < 10 {
		headerRows = 1
	}
	if diagnostic != "" {
		errorLine = forms.Fit("! "+forms.Safe(diagnostic), width, max(1, height-footerRows-headerRows-1), 0)
		errorRows = strings.Count(errorLine, "\n") + 1
		errorLine = s.Error.Render(errorLine)
	}
	destination := forms.Safe(e.Destination.String())
	if ansi.StringWidth(destination) > width-10 {
		destination = ansi.TruncateLeft(destination, ansi.StringWidth(destination)-max(1, width-11), "…")
	}
	saved := s.Label.Render(forms.Fit("Saved to: "+destination, width, 1, 0))
	rows := max(1, height-headerRows-footerRows-errorRows)
	body := ""
	if e.Picker {
		body = s.AlignedControlsView(e.Controls, e.Focus, width, rows)
	} else if rows >= 4 && width >= 24 {
		boundary := min(e.BasicsCount, len(e.Controls))
		left := e.Controls[:boundary]
		right := e.Controls[boundary:]
		paneTitle := "Inputs"
		if e.OpenInput != nil {
			paneTitle = "Input · " + forms.Safe(e.OpenInput.Name.String())
		} else if len(e.Inputs) == 0 {
			paneTitle = "Inputs · type {{name}} in Command"
		}
		if e.Section == 2 {
			paneTitle = "Computed values"
		}
		if e.Section == 3 {
			paneTitle = "Test · preview only"
		}
		tagsFocused := e.tagsFocused()
		render := func(controls []forms.Control, focus, w, h int, title string) string {
			if title == paneHelpTitle {
				return s.Frame(s.Focused.Bold(true).Render("Help · "+e.helpTopic())+"\n"+s.Text.Render(e.helpView(w-4, h-4))+"\n"+s.Label.Render("↑↓ scroll · F1 or Esc: close help"), w, h, true)
			}
			if title == "Tags" {
				// The checklist takes the right pane, so the form on the left stays
				// fully visible; it only takes keys after Enter on Tags.
				return s.Frame(s.Focused.Bold(true).Render(title)+"\n"+e.tagMenu(s, w-4, h-3), w, h, e.TagEditing)
			}
			content := s.Focused.Bold(true).Render(title) + "\n" + s.AlignedControlsView(e.withTagDisplay(controls, focus), focus, w-4, max(1, h-3))
			if title == "Test · preview only" && e.Test != nil {
				content = e.testContent(controls, focus, w-4, h-2)
			}
			return s.Frame(content, w, h, focus >= 0)
		}
		if len(e.Controls) > 0 && e.Controls[min(e.Focus, len(e.Controls)-1)].ID == tagsID {
			paneTitle = "Tags"
		}
		if e.PaneHelp {
			paneTitle = paneHelpTitle
		}
		leftFocus, rightFocus := -1, -1
		if e.Focus < boundary {
			leftFocus = e.Focus
		} else {
			rightFocus = e.Focus - boundary
		}
		if width >= 90 {
			leftWidth := width / 2
			body = lipgloss.JoinHorizontal(lipgloss.Top, render(left, leftFocus, leftWidth, rows, "Command · details"), render(right, rightFocus, width-leftWidth, rows, paneTitle))
		} else if rows >= 10 {
			leftRows := rows / 2
			body = render(left, leftFocus, width, leftRows, "Command · details") + "\n" + render(right, rightFocus, width, rows-leftRows-1, paneTitle)
		} else {
			// A bounded single frame retains both command and input context at the size guardrail.
			title := "Command · inputs"
			content := s.Focused.Render(title) + "\n" + s.AlignedControlsView(e.withTagDisplay(e.Controls, e.Focus), e.Focus, width-4, max(1, rows-3))
			if tagsFocused {
				content = s.Focused.Render("Tags") + "\n" + e.tagMenu(s, width-4, rows-3)
			}
			if e.PaneHelp {
				content = s.Focused.Render("Help · "+e.helpTopic()) + "\n" + e.helpView(width-4, rows-4)
			}
			if e.Section == 3 && e.Test != nil {
				content = e.testContent(e.Controls, e.Focus, width-4, rows-2)
			}
			body = s.Frame(content, width, rows, true)
		}
	} else {
		if e.Section == 3 && e.Test != nil {
			body = e.testContent(e.Controls, e.Focus, width, rows)
		} else {
			body = s.AlignedControlsView(e.Controls, e.Focus, width, rows)
		}
	}
	view := ""
	if headerRows == 2 {
		view = titleWithVersion(s, header, width) + "\n"
	}
	if headerRows > 0 {
		view += saved + "\n"
	}
	view += forms.Fit(body, width, rows, 0) + "\n"
	if errorRows > 0 {
		view += errorLine + "\n"
	}
	return s.Surface(view+s.Help.Render(footer), width, height)
}
