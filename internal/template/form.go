package template

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/regex"
	"github.com/samling/command-snippets/internal/templating"
)

type Form struct {
	Styles      *Styles
	Template    *templating.Template
	Texts       map[string]*Text
	Toggles     map[string]*bool
	Choices     map[string]*string
	Repeats     map[string][]*Text
	Focus       int
	Result      templating.Result
	Controls    []Control
	RegexScroll int
	RegexHidden bool
}

func NewForm(snippet models.Snippet, presets map[string]any) (*Form, error) {
	compiled, err := templating.Compile(snippet)
	if err != nil {
		return nil, err
	}
	form := &Form{Template: compiled, Texts: map[string]*Text{}, Toggles: map[string]*bool{}, Choices: map[string]*string{}, Repeats: map[string][]*Text{}}
	for _, in := range snippet.Inputs {
		value := in.DefaultValue()
		if preset, ok := presets[in.Name]; ok {
			value = preset
		}
		switch in.InputKind() {
		case "toggle":
			v, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("%s must be boolean", in.Name)
			}
			form.Toggles[in.Name] = &v
		case "choice":
			v, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("%s must be a choice label", in.Name)
			}
			form.Choices[in.Name] = &v
		case "repeat":
			items, ok := value.([]string)
			if !ok {
				return nil, fmt.Errorf("%s must be a list", in.Name)
			}
			for _, item := range items {
				v := NewText(item, false)
				form.Repeats[in.Name] = append(form.Repeats[in.Name], &v)
			}
		default:
			v, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("%s must be text", in.Name)
			}
			text := NewText(v, false)
			form.Texts[in.Name] = &text
		}
	}
	for name := range presets {
		found := false
		for _, in := range snippet.Inputs {
			if in.Name == name {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown input %s", name)
		}
	}
	form.Refresh()
	return form, nil
}
func (f *Form) Values() map[string]any {
	values := map[string]any{}
	for name, text := range f.Texts {
		values[name] = text.String()
	}
	for name, value := range f.Toggles {
		values[name] = *value
	}
	for name, value := range f.Choices {
		values[name] = *value
	}
	for _, in := range f.Template.Snippet.Inputs {
		if in.InputKind() == "repeat" {
			items := []string{}
			for _, text := range f.Repeats[in.Name] {
				items = append(items, text.String())
			}
			values[in.Name] = items
		}
	}
	return values
}
func (f *Form) Refresh() {
	focused := ""
	if f.Focus < len(f.Controls) && f.Focus >= 0 {
		focused = f.Controls[f.Focus].ID
	}
	f.Result = f.Template.Preview(f.Values())
	f.Controls = nil
	for _, in := range f.Template.Snippet.Inputs {
		if !f.Result.Visible[in.Name] {
			continue
		}
		label := in.DisplayName()
		// An empty required value is marked once in place; it is not an error.
		message, missing := f.Result.Errors[in.Name], false
		if message == requiredMessage {
			message, missing = "", true
		}
		control := Control{ID: in.Name, Label: label, Help: in.Help, Error: message, Missing: missing}
		switch in.InputKind() {
		case "toggle":
			control.Toggle = f.Toggles[in.Name]
		case "choice":
			control.Selection = f.Choices[in.Name]
			for _, choice := range in.Choices {
				control.Options = append(control.Options, choice.Label)
			}
		case "repeat":
			for idx, text := range f.Repeats[in.Name] {
				idx := idx
				f.Controls = append(f.Controls, Control{ID: fmt.Sprintf("%s-%d", in.Name, idx), Label: fmt.Sprintf("%s item %d", label, idx+1), Text: text, Help: "Each row is one item, including spaces. Ctrl-D removes this row.", Error: control.Error, Missing: control.Missing}, Control{ID: fmt.Sprintf("%s-remove-%d", in.Name, idx), Label: "Remove item", Action: func() { f.Repeats[in.Name] = append(f.Repeats[in.Name][:idx], f.Repeats[in.Name][idx+1:]...) }})
			}
			f.Controls = append(f.Controls, Control{ID: in.Name, Label: "Add " + label + " item", Help: in.Help, Error: control.Error, Missing: control.Missing, Action: func() { text := NewText("", false); f.Repeats[in.Name] = append(f.Repeats[in.Name], &text) }})
			continue
		default:
			control.Text = f.Texts[in.Name]
		}
		f.Controls = append(f.Controls, control)
	}
	// Repeat controls end with Add/Remove actions, so provide a separate
	// Enter submission target without changing those actions.
	if len(f.Controls) > 0 && f.Controls[len(f.Controls)-1].Action != nil {
		f.Controls = append(f.Controls, Control{ID: "$submit", Label: "Insert command", Submit: true})
	}
	for i, control := range f.Controls {
		if control.ID == focused {
			f.Focus = i
			break
		}
	}
	f.Focus = max(0, min(f.Focus, max(0, len(f.Controls)-1)))
}
func (f *Form) Update(key tea.KeyMsg) bool {
	if f.Focus >= 0 && f.Focus < len(f.Controls) {
		for _, in := range f.Template.Snippet.Inputs {
			if in.Name == f.Controls[f.Focus].ID && in.Validate != nil && in.Validate.Regex {
				switch key.String() {
				case "ctrl+r":
					f.RegexHidden = !f.RegexHidden
					return false
				case "ctrl+u":
					f.RegexScroll = max(0, f.RegexScroll-5)
					return false
				case "ctrl+d":
					f.RegexScroll += 5
					return false
				}
			}
		}
	}
	switch key.String() {
	case "ctrl+s":
		return f.submit()
	case "tab", "down":
		if len(f.Controls) > 0 {
			f.Focus = (f.Focus + 1) % len(f.Controls)
		}
	case "shift+tab", "up":
		if len(f.Controls) > 0 {
			f.Focus = (f.Focus + len(f.Controls) - 1) % len(f.Controls)
		}
	case "pgup":
		f.RegexScroll = max(0, f.RegexScroll-5)
	case "pgdown":
		f.RegexScroll += 5
	case "enter":
		if len(f.Controls) == 0 {
			return f.submit()
		}
		if f.Controls[f.Focus].Action != nil {
			f.Controls[f.Focus].Action()
			f.Refresh()
			return false
		}
		if f.Focus == len(f.Controls)-1 {
			return f.submit()
		}
		f.Focus++
	default:
		if len(f.Controls) > 0 {
			if key.String() == "ctrl+d" {
				for _, in := range f.Template.Snippet.Inputs {
					for idx := range f.Repeats[in.Name] {
						if f.Controls[f.Focus].ID == fmt.Sprintf("%s-%d", in.Name, idx) {
							f.Repeats[in.Name] = append(f.Repeats[in.Name][:idx], f.Repeats[in.Name][idx+1:]...)
							f.Refresh()
							return false
						}
					}
				}
			}
			f.Controls[f.Focus].Update(key)
			f.Refresh()
		}
	}
	return false
}

// NonfieldErrors is shared by the input form and editor Test header. Visible
// input errors stay in their controls; all other failures need an actionable reason.
func (f *Form) NonfieldErrors() string {
	visible := map[string]bool{}
	for _, in := range f.Template.Snippet.Inputs {
		visible[in.Name] = f.Result.Visible[in.Name]
	}
	keys := []string{}
	for key := range f.Result.Errors {
		if !visible[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	diagnostics := []string{}
	for _, key := range keys {
		diagnostics = append(diagnostics, Safe(key+": "+f.Result.Errors[key]))
	}
	return strings.Join(diagnostics, "\n")
}

// CommandPreview renders partial fragments for display only; executable output
// remains gated by Result.Valid and Result.Command. A part still waiting on
// inputs shows those inputs' names (‹pattern›), and every part derived from
// the focused input is underlined, so each field visibly maps to its piece.
func (f *Form) CommandPreview() string {
	s := f.Styles
	if s == nil {
		s = DefaultStyles()
	}
	names := map[string]string{}
	for _, in := range f.Template.Snippet.Inputs {
		names[in.Name] = in.DisplayName()
	}
	focused := ""
	if f.Focus >= 0 && f.Focus < len(f.Controls) {
		// Repeat item/action rows share their input's ID prefix; identifiers cannot contain '-'.
		focused, _, _ = strings.Cut(f.Controls[f.Focus].ID, "-")
	}
	var preview strings.Builder
	for _, part := range f.Result.Preview {
		text := Safe(part.Text)
		if part.Unfilled && len(part.Missing) > 0 {
			labels := make([]string, len(part.Missing))
			for i, name := range part.Missing {
				labels[i] = "‹" + Safe(names[name]) + "›"
			}
			text = strings.Join(labels, " ")
		}
		style := s.InputPreview
		switch {
		case part.Unfilled:
			style = s.InputUnfilled
		case part.Value:
			style = s.InputFilled
		}
		if focused != "" && slices.Contains(part.Inputs, focused) {
			style = style.Underline(true)
		}
		preview.WriteString(style.Render(text))
	}
	return preview.String()
}

// submit inserts a complete command; otherwise it moves focus to the first
// field that needs attention, so an incomplete form explains itself in place.
func (f *Form) submit() bool {
	if f.Result.Valid() {
		return true
	}
	for i, c := range f.Controls {
		if c.Missing || c.Error != "" {
			f.Focus = i
			break
		}
	}
	return false
}

// FieldErrors groups visible fields' errors by message, e.g. "b, c: is required".
// Unfocused rows only carry an error mark, so this names each blocking input
// once; skip is the control index whose error is already shown elsewhere
// (the focus hint), or -1.
func (f *Form) FieldErrors(skip int) string {
	skipped := ""
	if skip >= 0 && skip < len(f.Controls) {
		skipped, _, _ = strings.Cut(f.Controls[skip].ID, "-")
	}
	reported := map[string]bool{}
	groups := map[string][]string{}
	order := []string{}
	for _, c := range f.Controls {
		// Repeat item/action rows share their input's ID prefix; identifiers cannot contain '-'.
		input, _, _ := strings.Cut(c.ID, "-")
		if c.Error == "" || reported[input] || input == skipped {
			continue
		}
		reported[input] = true
		message := Safe(c.Error)
		if _, ok := groups[message]; !ok {
			order = append(order, message)
		}
		groups[message] = append(groups[message], strings.TrimSuffix(Safe(c.Label), " *"))
	}
	lines := make([]string, 0, len(order))
	for _, message := range order {
		lines = append(lines, strings.Join(groups[message], ", ")+": "+message)
	}
	return strings.Join(lines, "\n")
}

func (f *Form) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	s := f.Styles
	if s == nil {
		s = stylesFor(lipgloss.DefaultRenderer(), models.Settings{})
	}
	pattern := ""
	focusedRegex := false
	if f.Focus >= 0 && f.Focus < len(f.Controls) {
		id := f.Controls[f.Focus].ID
		for _, in := range f.Template.Snippet.Inputs {
			if in.Name == id && in.Validate != nil && in.Validate.Regex && f.Texts[id] != nil {
				focusedRegex = true
				if !f.RegexHidden && width >= 100 && height >= 10 {
					pattern = f.Texts[id].String()
				}
			}
		}
	}
	enter := "next"
	if len(f.Controls) == 0 || f.Focus == len(f.Controls)-1 {
		enter = "submit"
	} else if f.Controls[f.Focus].Action != nil {
		enter = "activate"
	}
	help := "Tab/↑↓:next Enter:" + enter
	if height < 8 {
		help = "Enter:" + enter
	}
	if width >= 48 && height >= 14 {
		target := "Enter on last input"
		if len(f.Controls) == 0 {
			target = "Enter"
		}
		if len(f.Controls) > 0 && f.Controls[len(f.Controls)-1].Submit {
			target = "Enter on Insert command"
		}
		help = "Insert command: " + target + " · Ctrl-S:insert\n" + help
		if width >= 90 && len(f.Controls) > 0 {
			c := f.Controls[f.Focus]
			if c.Selection != nil || c.Toggle != nil {
				help += " ←→:select"
			} else if c.Text != nil {
				help += " ←→:cursor Home/End:jump Ctrl-X:clear"
			}
		}
	}
	if focusedRegex && width >= 70 && height >= 12 {
		help += "\nCtrl-R:pane Ctrl-U/D or PgUp/PgDn:scroll"
	}
	footer := s.InputHelp.Render(Fit(help+"\nEsc:back Ctrl-C:cancel", width, max(1, height-1), 0))
	footerRows := strings.Count(footer, "\n") + 1
	if width < 48 || height < 14 {
		rows := height - footerRows
		content := f.compactView(s, width, rows)
		if pattern != "" {
			leftWidth := (width - 1) / 2
			rightWidth := width - leftWidth - 1
			explanation := Fit(Safe(regex.ExplainRegexPattern(pattern)), rightWidth-4, rows-3, f.RegexScroll)
			explanation = s.Renderer.NewStyle().Foreground(s.InputRegexPanel.GetForeground()).Render(explanation)
			pane := f.inputPanel(s, s.InputRegexTitle.Render("Pattern Explanation"), explanation, rightWidth, rows)
			content = lipgloss.JoinHorizontal(lipgloss.Top, s.Renderer.NewStyle().Width(leftWidth).Render(f.compactView(s, leftWidth, rows)), " ", pane)
		}
		return s.Surface(content+"\n"+footer, width, height)
	}

	header := Fit("Template: "+Safe(f.Template.Snippet.Name), width, 1, 0)
	rows := height - footerRows - 1
	hintText := ""
	inputs := func(w, h int) string {
		content := s.InputControls.Label.Render("No inputs. Insert command [Enter]")
		if len(f.Controls) > 0 {
			content, hintText = alignedView(f.Controls, f.Focus, w-4, h-3, s.InputControls)
		}
		return f.inputPanel(s, s.InputControls.Focused.Render(Fit("Inputs", w-4, 1, 0)), content, w, h)
	}
	preview := func(w, h int) string {
		// Budget rows so neither part can push the other out of a short panel:
		// blocking reasons keep at least one row and the command keeps the rest.
		command := f.CommandPreview()
		skip := -1
		if f.Focus >= 0 && f.Focus < len(f.Controls) && f.Controls[f.Focus].Error != "" && strings.Contains(hintText, strings.Join(strings.Fields("! "+Safe(f.Controls[f.Focus].Error)), " ")) {
			skip = f.Focus
		}
		diagnostics := strings.Trim(f.NonfieldErrors()+"\n"+f.FieldErrors(skip), "\n")
		content := command
		if avail := h - 3; diagnostics != "" && avail > 0 {
			diagRows := min(strings.Count(Fit(diagnostics, w-4, avail, 0), "\n")+1, max(1, avail-1))
			commandRows := avail - diagRows
			content = Fit(command, w-4, max(0, commandRows), 0)
			if commandRows <= 0 {
				content = ""
			}
			content = strings.TrimPrefix(content+"\n"+s.InputControls.Error.Render(Fit(diagnostics, w-4, diagRows, 0)), "\n")
		}
		return f.inputPanel(s, s.InputPreviewTitle.Render("Live preview"), content, w, h)
	}
	var panels string
	if width >= 90 {
		leftWidth := (width - 1) / 2
		rightWidth := width - leftWidth - 1
		left := inputs(leftWidth, rows)
		right := preview(rightWidth, rows)
		if pattern != "" {
			previewRows := max(5, rows/3)
			regexRows := rows - previewRows
			explanation := Fit(Safe(regex.ExplainRegexPattern(pattern)), rightWidth-4, regexRows-3, f.RegexScroll)
			explanation = s.Renderer.NewStyle().Foreground(s.InputRegexPanel.GetForeground()).Render(explanation)
			right = preview(rightWidth, previewRows) + "\n" + f.inputPanel(s, s.InputRegexTitle.Render("Pattern Explanation"), explanation, rightWidth, regexRows)
		}
		panels = lipgloss.JoinHorizontal(lipgloss.Top, left, s.Renderer.NewStyle().Width(1).Render(""), right)
	} else {
		previewRows := max(5, rows/3)
		panels = inputs(width, rows-previewRows) + "\n" + preview(width, previewRows)
	}
	return s.Surface(header+"\n"+panels+"\n"+footer, width, height)
}

// inputPanel uses the form palette without changing library/editor controls.
func (f *Form) inputPanel(s *Styles, title, content string, width, height int) string {
	return s.InputRegexPanel.UnsetForeground().Width(width - 2).Height(height - 2).Render(title + "\n" + Fit(content, width-4, height-3, 0))
}

// requiredMessage is templating's error for an empty required value; the form
// shows it as a quiet in-place marker instead of an error.
const requiredMessage = "is required"

// compactView keeps the focused value and insertion failure ahead of decoration.
func (f *Form) compactView(s *Styles, width, height int) string {
	if height <= 0 {
		return ""
	}
	diagnostic := f.NonfieldErrors()
	if diagnostic == "" && !f.Result.Valid() {
		if f.Focus >= 0 && f.Focus < len(f.Controls) {
			diagnostic = Safe(f.Controls[f.Focus].Error)
		}
		if diagnostic == "" {
			keys := make([]string, 0, len(f.Result.Errors))
			for key, message := range f.Result.Errors {
				if message != requiredMessage {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				diagnostic = Safe(keys[0] + ": " + f.Result.Errors[keys[0]])
			}
		}
	}
	diagnosticRows := 0
	if diagnostic != "" {
		diagnosticRows = min(strings.Count(Fit(diagnostic, width, height, 0), "\n")+1, max(1, height-1))
	}
	bodyRows := height - diagnosticRows
	lines := []string{}
	if bodyRows >= 4 {
		lines = append(lines, Fit("Template: "+Safe(f.Template.Snippet.Name), width, 1, 0))
		bodyRows--
	}
	previewRows := 0
	if bodyRows >= 5 {
		previewRows = min(3, bodyRows-2)
		bodyRows -= previewRows
	}
	controls := []string{}
	for i, c := range f.Controls {
		prefix := "  "
		labelStyle := s.InputControls.Label
		if i == f.Focus {
			prefix = "> "
			labelStyle = s.InputControls.Focused
		}
		label := ansi.Truncate(Safe(c.Label), max(1, width/2-3), "…") + ":"
		valueWidth := max(1, width-ansi.StringWidth(prefix+label)-1)
		value := "[Enter]"
		switch {
		case c.Text != nil && i != f.Focus && c.Missing && len(c.Text.Value) == 0:
			value = s.InputControls.Label.Render("required")
		case c.Text != nil && c.Missing && len(c.Text.Value) == 0 && valueWidth > 10:
			value = requiredPlaceholder(s.InputControls.Label, s.Renderer)
		case c.Text != nil:
			value = s.InputControls.Value.Render(c.Text.OneLineWithRenderer(valueWidth, i == f.Focus, s.Renderer))
		case c.Selection != nil:
			value = s.InputControls.Selected.Render(ansi.Truncate("<"+Safe(*c.Selection)+">", valueWidth, ""))
		case c.Toggle != nil:
			value = "[off]"
			if *c.Toggle {
				value = "[on]"
			}
		}
		if c.Text == nil {
			value = s.InputControls.Value.Render(value)
		}
		controls = append(controls, prefix+labelStyle.Render(label)+" "+value)
	}
	if len(controls) == 0 {
		controls = append(controls, s.InputControls.Focused.Render("Insert command [Enter]"))
	}
	lines = append(lines, Fit(strings.Join(controls, "\n"), width, max(1, bodyRows), max(0, f.Focus-max(0, bodyRows/3))))
	if diagnosticRows > 0 {
		lines = append(lines, s.InputControls.Error.Render(Fit(diagnostic, width, diagnosticRows, 0)))
	}
	if previewRows > 0 {
		lines = append(lines, s.InputPreviewTitle.Render("Live preview")+"\n"+Fit(f.CommandPreview(), width, previewRows-1, 0))
	}
	return s.Renderer.NewStyle().Height(height).Render(strings.Join(lines, "\n"))
}
