package template

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/regex"
	"github.com/samling/command-snippets/internal/templating"
)

type Form struct {
	Template    *templating.Template
	Texts       map[string]*Text
	Toggles     map[string]*bool
	Choices     map[string]*string
	Repeats     map[string][]*Text
	Focus       int
	Result      templating.Result
	Controls    []Control
	RegexScroll int
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
		if f.Result.Required[in.Name] {
			label += " *"
		}
		control := Control{ID: in.Name, Label: label, Help: in.Help, Error: f.Result.Errors[in.Name]}
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
				f.Controls = append(f.Controls, Control{ID: fmt.Sprintf("%s-%d", in.Name, idx), Label: fmt.Sprintf("%s item %d", label, idx+1), Text: text, Help: "Each row is one item, including spaces. Ctrl-D removes this row.", Error: control.Error}, Control{ID: fmt.Sprintf("%s-remove-%d", in.Name, idx), Label: "Remove item", Action: func() { f.Repeats[in.Name] = append(f.Repeats[in.Name][:idx], f.Repeats[in.Name][idx+1:]...) }})
			}
			f.Controls = append(f.Controls, Control{ID: in.Name, Label: "Add " + label + " item", Help: in.Help, Error: control.Error, Action: func() { text := NewText("", false); f.Repeats[in.Name] = append(f.Repeats[in.Name], &text) }})
			continue
		default:
			control.Text = f.Texts[in.Name]
		}
		f.Controls = append(f.Controls, control)
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
	switch key.String() {
	case "ctrl+s":
		return f.Result.Valid()
	case "tab":
		if len(f.Controls) > 0 {
			f.Focus = (f.Focus + 1) % len(f.Controls)
		}
	case "shift+tab":
		if len(f.Controls) > 0 {
			f.Focus = (f.Focus + len(f.Controls) - 1) % len(f.Controls)
		}
	case "pgup":
		f.RegexScroll = max(0, f.RegexScroll-5)
	case "pgdown":
		f.RegexScroll += 5
	case "enter":
		if len(f.Controls) == 0 {
			return f.Result.Valid()
		}
		if f.Controls[f.Focus].Action != nil {
			f.Controls[f.Focus].Action()
			f.Refresh()
			return false
		}
		if f.Focus == len(f.Controls)-1 {
			return f.Result.Valid()
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
func (f *Form) View(width, height int) string {
	header := "Fill inputs: " + Safe(f.Template.Snippet.Name) + "\nLive preview: "
	if f.Result.Valid() {
		header += Safe(f.Result.Command)
	} else {
		header += "[invalid — complete the fields below]"
	}
	diagnostics := f.NonfieldErrors()
	// Prioritize the reason within the existing header when space is scarce.
	if diagnostics != "" {
		header = diagnostics + "\nFill inputs: " + Safe(f.Template.Snippet.Name)
	}
	footer := Fit("Tab:next Ctrl-S:use\nEsc:back Ctrl-C:cancel", width, max(1, height), 0)
	footerRows := strings.Count(footer, "\n") + 1
	headerRows := min(max(1, height/3), max(1, height-footerRows-1))
	if diagnostics != "" {
		headerRows = max(1, height-footerRows-1)
	}
	top := Fit(header, width, headerRows, 0)
	bodyRows := max(1, height-footerRows-strings.Count(top, "\n")-1)
	content := ControlsView(f.Controls, f.Focus, width, bodyRows)
	if bodyRows >= 4 {
		for _, in := range f.Template.Snippet.Inputs {
			if in.Validate != nil && in.Validate.Regex && f.Result.Visible[in.Name] {
				if text := f.Texts[in.Name]; text != nil {
					regexRows := max(1, bodyRows/2)
					explanation := Fit(Safe(regex.ExplainRegexPattern(text.String())), width, regexRows, f.RegexScroll)
					content = ControlsView(f.Controls, f.Focus, width, max(1, bodyRows-regexRows-1)) + "\nRegex (PgUp/PgDn):\n" + explanation
					break
				}
			}
		}
	}
	return top + "\n" + content + "\n" + footer
}
