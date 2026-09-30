package workspace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
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
	Entry                                   *library.Entry
	Name, Description, Command, Destination forms.Text
	Tags                                    []forms.Text
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
	e := &Editor{Entry: entry, Name: text(snippet.Name), Description: text(snippet.Description), Command: forms.NewText(snippet.Command, true), Destination: text(destination), PickerName: text("input"), PickerFlag: text(""), PickerKind: "text", TestValues: map[string]any{}}
	for _, tag := range snippet.Tags {
		e.Tags = append(e.Tags, text(tag))
	}
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
	for _, tag := range e.Tags {
		if strings.TrimSpace(tag.String()) != "" {
			s.Tags = append(s.Tags, strings.TrimSpace(tag.String()))
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
	if len(e.Expressions) > 0 {
		s.Expressions = map[string]string{}
		for _, p := range e.Expressions {
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
	e.Inputs = append(e.Inputs, newInput(models.Input{Name: name}))
}
func (e *Editor) Build() {
	e.Controls = nil
	addText := func(label string, t *forms.Text, help string) {
		e.Controls = append(e.Controls, forms.Control{Label: label, Text: t, Help: help})
	}
	addBool := func(label string, b *bool) { e.Controls = append(e.Controls, forms.Control{Label: label, Toggle: b}) }
	addAction := func(label string, action func()) {
		e.Controls = append(e.Controls, forms.Control{Label: label, Action: action})
	}
	addRows := func(label string, rows *[]forms.Text) {
		for i := range *rows {
			i := i
			addText(fmt.Sprintf("%s %d", label, i+1), &(*rows)[i], "")
			addAction("Remove "+label, func() { *rows = append((*rows)[:i], (*rows)[i+1:]...) })
		}
		addAction("Add "+label, func() { *rows = append(*rows, text("")) })
	}
	addPairs := func(label, left, right string, rows *[]pairDraft) {
		for i := range *rows {
			i := i
			addText(label+" "+left, &(*rows)[i].Key, "")
			addText(label+" "+right, &(*rows)[i].Value, "")
			addAction("Remove "+label, func() { *rows = append((*rows)[:i], (*rows)[i+1:]...) })
		}
		addAction("Add "+label, func() { *rows = append(*rows, pairDraft{text(""), text("")}) })
	}
	addCondition := func(label string, d *conditionDraft) {
		e.Controls = append(e.Controls, forms.Control{Label: label, Selection: &d.Mode, Options: []string{"none", "equals", "not_equals", "expr"}})
		if d.Mode == "none" {
			return
		}
		if d.Mode == "expr" {
			addText(label+" expression", &d.Expr, "Boolean expression, e.g. inputs.mode == \"Named\".")
			return
		}
		addText(label+" input name", &d.Input, "A declared controller input; hidden values use their typed zero.")
		kind := "text"
		for _, in := range e.Inputs {
			if in.Name.String() == d.Input.String() {
				kind = in.Kind
			}
		}
		switch kind {
		case "toggle":
			addBool(label+" value", &d.Boolean)
		case "repeat":
			addRows(label+" item", &d.Items)
		default:
			addText(label+" value", &d.Value, "Exact raw value, not its emitted shell fragment.")
		}
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
	switch e.Section {
	case 0:
		addText("Friendly name", &e.Name, "A searchable display title, not a slug.")
		addText("Description", &e.Description, "")
		addRows("Tag", &e.Tags)
		addText("Command", &e.Command, "Alt-Enter inserts a newline. Ctrl-P marks start/end and creates an input; literal shell syntax is unchanged.")
		for _, name := range templating.Placeholders(e.Command.String()) {
			name := name
			var existing *inputDraft
			expression := false
			for _, in := range e.Inputs {
				if in.Name.String() == name {
					existing = in
				}
			}
			for _, p := range e.Expressions {
				if p.Key.String() == name {
					expression = true
				}
			}
			if expression {
				continue
			}
			label := "Configure {{" + name + "}}"
			if existing == nil {
				label = "Create undefined input {{" + name + "}}"
			}
			addAction(label, func() {
				if existing == nil {
					existing = newInput(models.Input{Name: name})
					e.Inputs = append(e.Inputs, existing)
				}
				e.Section = 1
				e.Build()
				for i, control := range e.Controls {
					if control.Text == &existing.Name {
						e.Focus = i
						break
					}
				}
			})
		}
		if e.Entry == nil {
			addText("Destination", &e.Destination, "Main file, an included source, or a path matched by an include pattern.")
		}
	case 1:
		for idx, d := range e.Inputs {
			idx := idx
			addText(fmt.Sprintf("Input %d name", idx+1), &d.Name, "")
			addText("Label", &d.Label, "")
			addText("Help", &d.Help, "")
			e.Controls = append(e.Controls, forms.Control{Label: "Behavior", Selection: &d.Kind, Options: []string{"text", "flag", "toggle", "choice", "repeat"}})
			if d.Kind == "flag" || d.Kind == "toggle" || d.Kind == "repeat" {
				addText("Flag", &d.Flag, "")
			}
			addBool("Required", &d.Required)
			addBool("Use default", &d.UseDefault)
			if d.UseDefault {
				switch d.Kind {
				case "toggle":
					addBool("Default", &d.ToggleDefault)
				case "repeat":
					addRows("Default item", &d.RepeatDefault)
				default:
					addText("Default", &d.Default, "")
				}
			}
			if d.Kind == "choice" {
				addPairs("Choice", "label", "output", &d.Choices)
			}
			if d.Kind == "text" || d.Kind == "flag" || d.Kind == "choice" {
				addPairs("Special", "raw value", "output", &d.Special)
			}
			if d.Kind != "toggle" {
				addText("Validation pattern", &d.Pattern, "Go RE2 pattern applied to nonempty values.")
				addBool("Integer range", &d.UseRange)
				if d.UseRange {
					addText("Minimum", &d.Min, "")
					addText("Maximum", &d.Max, "")
				}
				addBool("Value must be a regex", &d.Regex)
			}
			addCondition("Visible when", &d.Visible)
			addCondition("Required when", &d.RequiredWhen)
			addAction("Remove this input", func() { e.Inputs = append(e.Inputs[:idx], e.Inputs[idx+1:]...) })
		}
		addAction("Add input", e.addInput)
	case 2:
		addPairs("Expression", "name", "string result", &e.Expressions)
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
	}
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
func (e *Editor) View(width, height int) string {
	titles := []string{"Basics", "Inputs", "Advanced", "Test"}
	status := "clean"
	if e.Dirty() {
		status = "unsaved"
	}
	header := "Edit command: " + titles[e.Section]
	if status == "unsaved" {
		header += " *"
	}
	if e.Picker {
		header = "Mark input — confirm the exact selected range\n"
	}
	header = forms.Fit(header, width, 1, 0)
	footer := forms.Fit("Ctrl-S:save Esc:back\nCtrl-arrows:section F1", width, height, 0)
	footerRows := strings.Count(footer, "\n") + 1
	errorRows := 0
	errorLine := ""
	if e.Error != "" {
		errorRows = 1
		errorLine = "! " + forms.Safe(e.Error)
	}
	bodyRows := max(1, height-1-footerRows-errorRows)
	body := forms.ControlsView(e.Controls, e.Focus, width, bodyRows)
	if e.Section == 3 && e.Test != nil && !e.Picker {
		preview := "Preview: [invalid / incomplete]"
		if e.Test.Result.Valid() {
			preview = "Preview: " + forms.Safe(e.Test.Result.Command)
		}
		previewRows := min(max(1, bodyRows/3), max(1, bodyRows-1))
		if diagnostics := e.Test.NonfieldErrors(); diagnostics != "" {
			preview = diagnostics
			previewRows = bodyRows
		}
		body = forms.Fit(preview, width, previewRows, 0) + "\n" + forms.ControlsView(e.Test.Controls, e.Test.Focus, width, max(1, bodyRows-previewRows))
	}
	view := header + "\n" + forms.Fit(body, width, bodyRows, 0) + "\n"
	if errorRows > 0 {
		view += forms.Fit(errorLine, width, 1, 0) + "\n"
	}
	return view + footer
}
