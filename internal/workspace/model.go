package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/search"
	forms "github.com/samling/command-snippets/internal/template"
	"github.com/samling/command-snippets/internal/templating"
)

type Options struct {
	Start   string
	Entry   *library.Entry
	Presets map[string]any
	NoColor bool
}
type Model struct {
	Library                                         *library.Library
	Index                                           *search.Index
	Query                                           forms.Text
	Tags                                            []string
	Untagged                                        bool
	Matches                                         []search.Match
	Selected, Category, Focus, DetailScroll         int
	Width, Height                                   int
	ShowDetail, Help, NoColor                       bool
	NoColorOverride                                 bool
	Mode                                            string
	Form                                            *forms.Form
	Editor                                          *Editor
	Settings                                        *SettingsEditor
	Command, Message                                string
	Cancelled, Done, ConfirmDiscard, DiscardCancels bool
}

func New(lib *library.Library, options Options) (*Model, error) {
	m := &Model{Library: lib, Query: forms.NewText("", false), Width: 80, Height: 24, Mode: "library", NoColor: options.NoColor || lib.Settings.Color == "never", NoColorOverride: options.NoColor}
	m.reloadIndex()
	if lib.Error() != nil {
		m.Mode = "recovery"
	}
	if options.Entry != nil {
		for i, match := range m.Matches {
			if match.Entry.Key == options.Entry.Key {
				m.Selected = i
			}
		}
	}
	var err error
	switch options.Start {
	case "add":
		if lib.Error() != nil {
			return m, nil
		}
		m.Editor, err = NewEditor(nil, m.destination())
		m.Mode = "editor"
	case "edit":
		if lib.Error() != nil {
			return m, nil
		}
		if options.Entry != nil {
			m.Editor, err = NewEditor(options.Entry, "")
			m.Mode = "editor"
		} else {
			m.Message = "Select a command, then Ctrl-E to edit."
		}
	case "inputs":
		if lib.Error() != nil {
			return nil, lib.Error()
		}
		if options.Entry == nil {
			return nil, fmt.Errorf("missing selected command")
		}
		m.Form, err = forms.NewForm(options.Entry.Snippet, options.Presets)
		m.Mode = "inputs"
	}
	return m, err
}
func (m *Model) Init() tea.Cmd { return nil }
func (m *Model) destination() string {
	path := m.Library.Settings.DefaultSource
	if path == "" {
		path = m.Library.ConfigPath
	}
	return library.ExpandPath(path, filepath.Dir(m.Library.ConfigPath))
}
func (m *Model) reloadIndex() {
	m.NoColor = m.NoColorOverride || m.Library.Settings.Color == "never"
	forms.SetupColorProfile(m.NoColor)
	if !m.NoColor && m.Library.Settings.Color == "always" {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}
	key := ""
	if len(m.Matches) > 0 && m.Selected < len(m.Matches) {
		key = m.Matches[m.Selected].Entry.Key
	}
	m.Index = search.New(m.Library.Entries)
	validTags := map[string]bool{}
	for _, category := range m.Index.Categories {
		validTags[category.Key] = true
	}
	tags := []string{}
	for _, tag := range m.Tags {
		if validTags[tag] {
			tags = append(tags, tag)
		}
	}
	m.Tags = tags
	m.refresh(key)
}
func (m *Model) refresh(key string) {
	m.Matches = m.Index.Query(m.Query.String(), m.Tags, m.Untagged)
	m.Selected = max(0, min(m.Selected, max(0, len(m.Matches)-1)))
	if key != "" {
		for i, match := range m.Matches {
			if match.Entry.Key == key {
				m.Selected = i
				break
			}
		}
	}
	m.Category = max(0, min(m.Category, len(m.Index.Categories)+1))
}
func (m *Model) selected() *library.Entry {
	if len(m.Matches) == 0 || m.Selected >= len(m.Matches) {
		return nil
	}
	return m.Matches[m.Selected].Entry
}
func (m *Model) exit(cancel bool) (tea.Model, tea.Cmd) {
	m.Cancelled = cancel
	m.Done = true
	return m, tea.Quit
}
func (m *Model) back() {
	m.Mode = "library"
	if m.Library.Error() != nil {
		m.Mode = "recovery"
	}
	m.Form = nil
	m.Editor = nil
	m.Settings = nil
	m.ConfirmDiscard = false
}
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.Width = max(0, msg.Width)
		m.Height = max(0, msg.Height)
		return m, nil
	case tea.KeyMsg:
		key := msg.String()
		if m.ConfirmDiscard {
			switch key {
			case "y", "Y":
				m.ConfirmDiscard = false
				if m.DiscardCancels {
					return m.exit(true)
				}
				m.back()
			case "n", "N", "esc":
				m.ConfirmDiscard = false
			}
			return m, nil
		}
		if key == "f1" {
			m.Help = !m.Help
			return m, nil
		}
		if m.Help {
			if key == "ctrl+c" {
				m.Help = false
			} else {
				if key == "esc" {
					m.Help = false
				}
				return m, nil
			}
		}
		if key == "esc" || key == "ctrl+c" {
			if m.Mode == "editor" && m.Editor.Picker {
				m.Editor.Picker = false
				m.Editor.Build()
				return m, nil
			}
			dirty := m.Mode == "editor" && m.Editor.Dirty() || m.Mode == "settings" && m.Settings.Dirty()
			if dirty {
				m.ConfirmDiscard = true
				m.DiscardCancels = key == "ctrl+c"
				return m, nil
			}
			if key == "ctrl+c" || m.Mode == "library" || m.Mode == "recovery" {
				return m.exit(true)
			}
			m.back()
			return m, nil
		}
		if m.Width < 24 || m.Height < 6 {
			return m, nil
		}
		switch m.Mode {
		case "recovery":
			switch key {
			case "ctrl+r":
				m.Library = library.Load(m.Library.ConfigPath, m.Library.CWD)
				m.reloadIndex()
				if m.Library.Error() == nil {
					m.Mode = "library"
				}
			case "ctrl+o":
				if source := m.Library.Source(m.Library.ConfigPath); source != nil && source.Root != nil {
					m.Settings = NewSettingsEditor(m.Library)
					m.Mode = "settings"
				}
			}
		case "library":
			switch key {
			case "tab":
				m.Focus = (m.Focus + 1) % 4
			case "shift+tab":
				m.Focus = (m.Focus + 3) % 4
			case "ctrl+l":
				m.Query = forms.NewText("", false)
				m.Tags = nil
				m.Untagged = false
				m.Selected = 0
				m.refresh("")
			case "ctrl+r":
				m.Library = library.Load(m.Library.ConfigPath, m.Library.CWD)
				m.reloadIndex()
				if m.Library.Error() != nil {
					m.Mode = "recovery"
				}
			case "ctrl+n":
				var err error
				m.Editor, err = NewEditor(nil, m.destination())
				if err != nil {
					m.Message = err.Error()
				} else {
					m.Mode = "editor"
				}
			case "ctrl+e":
				if entry := m.selected(); entry != nil {
					var err error
					m.Editor, err = NewEditor(entry, "")
					if err != nil {
						m.Message = err.Error()
					} else {
						m.Mode = "editor"
					}
				}
			case "ctrl+o":
				m.Settings = NewSettingsEditor(m.Library)
				m.Mode = "settings"
			case "ctrl+d":
				m.ShowDetail = !m.ShowDetail
			case "up", "down":
				direction := 1
				if key == "up" {
					direction = -1
				}
				if m.Focus == 1 {
					m.Category = max(0, min(m.Category+direction, len(m.Index.Categories)+1))
				} else {
					m.Selected = max(0, min(m.Selected+direction, max(0, len(m.Matches)-1)))
					m.DetailScroll = 0
				}
			case "pgup", "pgdown":
				if m.Focus == 3 {
					direction := max(1, m.Height/2)
					if key == "pgup" {
						direction = -direction
					}
					m.DetailScroll = max(0, m.DetailScroll+direction)
				}
			case "enter", " ":
				if m.Focus == 1 {
					m.toggleCategory()
				} else if key == "enter" {
					entry := m.selected()
					if entry != nil {
						if len(entry.Snippet.Inputs) == 0 {
							command, err := entry.Template.Render(nil)
							if err != nil {
								m.Message = err.Error()
							} else {
								m.Command = command
								return m.exit(false)
							}
						} else {
							var err error
							m.Form, err = forms.NewForm(entry.Snippet, nil)
							if err != nil {
								m.Message = err.Error()
							} else {
								m.Mode = "inputs"
							}
						}
					}
				} else if m.Focus == 0 {
					m.Query.Update(msg)
					m.refresh("")
				}
			default:
				if m.Focus == 0 {
					if m.Query.Update(msg) {
						m.Selected = 0
						m.DetailScroll = 0
						m.refresh("")
					}
				}
			}
		case "inputs":
			if m.Form.Update(msg) {
				m.Command = m.Form.Result.Command
				return m.exit(false)
			}
		case "editor":
			e := m.Editor
			switch key {
			case "ctrl+s":
				m.saveEditor()
			case "ctrl+left", "ctrl+right":
				if !e.Picker {
					if e.Test != nil {
						e.TestValues = e.Test.Values()
					}
					direction := 1
					if key == "ctrl+left" {
						direction = -1
					}
					e.Section = (e.Section + direction + 4) % 4
					e.Focus = 0
					e.Build()
				}
			case "ctrl+p":
				if e.Section == 0 && len(e.Controls) > 0 && e.Controls[e.Focus].Text == &e.Command {
					e.Mark()
				}
			default:
				if e.Section == 3 && e.Test != nil && !e.Picker {
					e.Test.Update(msg)
					e.TestValues = e.Test.Values()
				} else {
					switch key {
					case "tab":
						if len(e.Controls) > 0 {
							e.Focus = (e.Focus + 1) % len(e.Controls)
						}
					case "shift+tab":
						if len(e.Controls) > 0 {
							e.Focus = (e.Focus + len(e.Controls) - 1) % len(e.Controls)
						}
					case "enter":
						if len(e.Controls) > 0 {
							if e.Controls[e.Focus].Action != nil {
								e.Test = nil
								e.Error = ""
								e.Controls[e.Focus].Action()
							} else {
								e.Focus = (e.Focus + 1) % len(e.Controls)
							}
						}
					default:
						if len(e.Controls) > 0 {
							if e.Controls[e.Focus].Update(msg) {
								e.Test = nil
								e.Error = ""
							}
						}
					}
					e.Build()
				}
			}
		case "settings":
			if key == "ctrl+s" {
				settings, err := m.Settings.Draft()
				if err != nil {
					m.Settings.Error = err.Error()
					break
				}
				next, err := m.Library.SaveSettings(settings)
				if next != nil {
					m.Library = next
					m.reloadIndex()
					m.back()
					if err != nil {
						m.Message = err.Error()
					} else {
						m.Message = "Settings saved."
					}
				} else if err != nil {
					m.Settings.Error = err.Error()
				}
			} else {
				m.Settings.Update(msg)
			}
		}
	}
	return m, nil
}
func (m *Model) saveEditor() {
	e := m.Editor
	if e.Picker {
		e.Error = "Confirm or cancel the marked input first."
		return
	}
	draft, err := e.Draft()
	if err != nil {
		e.Error = err.Error()
		return
	}
	compiled, err := templating.Compile(draft)
	if err != nil {
		e.Error = err.Error()
		return
	}
	test := compiled.Preview(e.validTestValues())
	incomplete := !test.Valid()
	for name, message := range test.Errors {
		if message != "is required" {
			e.Error = "Test " + name + ": " + message
			return
		}
	}
	next, saved, err := m.Library.Save(e.Entry, draft, e.Destination.String())
	if saved != nil {
		m.Library = next
		m.reloadIndex()
		m.refresh(saved.Key)
		m.back()
		if err != nil {
			m.Message = err.Error()
		} else if incomplete {
			m.Message = "Saved reusable definition; representative test incomplete (required values missing)."
		} else {
			m.Message = "Saved to " + saved.Source.DisplayPath
		}
	} else if err != nil {
		e.Error = err.Error()
	}
}
func (m *Model) toggleCategory() {
	switch m.Category {
	case 0:
		m.Tags = nil
		m.Untagged = false
	case 1:
		m.Untagged = !m.Untagged
		m.Tags = nil
	default:
		m.Untagged = false
		key := m.Index.Categories[m.Category-2].Key
		found := -1
		for i, tag := range m.Tags {
			if tag == key {
				found = i
			}
		}
		if found >= 0 {
			m.Tags = append(m.Tags[:found], m.Tags[found+1:]...)
		} else {
			m.Tags = append(m.Tags, key)
		}
	}
	m.Selected = 0
	m.refresh("")
}
func (m *Model) categories() string {
	rows := []string{"All commands", "Untagged"}
	for _, c := range m.Index.Categories {
		rows = append(rows, fmt.Sprintf("%s (%d)", c.Label, c.Count))
	}
	result := []string{"Categories — tags intersect"}
	for i, row := range rows {
		marker := "[ ]"
		if i == 0 && len(m.Tags) == 0 && !m.Untagged || i == 1 && m.Untagged {
			marker = "[x]"
		}
		if i > 1 {
			for _, tag := range m.Tags {
				if tag == m.Index.Categories[i-2].Key {
					marker = "[x]"
				}
			}
		}
		focus := " "
		if m.Focus == 1 && m.Category == i {
			focus = ">"
		}
		result = append(result, focus+marker+" "+forms.Safe(row))
	}
	return strings.Join(result, "\n")
}
func (m *Model) results(width, height int) string {
	if height <= 1 && len(m.Matches) > 0 {
		return forms.Fit("> "+forms.Safe(m.Matches[m.Selected].Entry.Snippet.Name), width, 1, 0)
	}
	if len(m.Matches) == 0 {
		if len(m.Library.Entries) == 0 {
			return "No commands yet. Ctrl-N: create a command."
		}
		return "No matches. Ctrl-L: clear search and filters."
	}
	start := max(0, m.Selected-max(0, height/2))
	end := min(len(m.Matches), start+max(1, height-1))
	counts := map[string]int{}
	for _, match := range m.Matches {
		counts[match.Entry.Snippet.Name]++
	}
	lines := []string{fmt.Sprintf("Commands (%d)", len(m.Matches))}
	for i := start; i < end; i++ {
		entry := m.Matches[i].Entry
		prefix := "  "
		if i == m.Selected {
			prefix = "> "
		}
		name := entry.Snippet.Name
		if counts[name] > 1 {
			name += " — " + entry.Source.DisplayPath
		}
		lines = append(lines, forms.Fit(prefix+forms.Safe(name), width, 1, 0))
	}
	return strings.Join(lines, "\n")
}
func (m *Model) detail() string {
	entry := m.selected()
	if entry == nil {
		return "Command detail"
	}
	s := entry.Snippet
	lines := []string{s.Name, s.Description, "Tags: " + strings.Join(s.Tags, ", "), "Source: " + entry.Source.DisplayPath}
	if entry.Source.Symlink {
		lines = append(lines, "Read-only symlink source")
	}
	lines = append(lines, "", "Literal command:", s.Command, "", "Inputs:")
	for _, in := range s.Inputs {
		required := ""
		if in.Required {
			required = " required"
		}
		lines = append(lines, fmt.Sprintf("%s [%s%s] default: %v", in.DisplayName(), in.InputKind(), required, in.DefaultValue()), in.Help)
	}
	return forms.Safe(strings.Join(lines, "\n"))
}
func (m *Model) View() string {
	if m.Done {
		return ""
	}
	if m.Width < 24 || m.Height < 6 {
		return forms.Fit("Resize terminal or Esc to cancel.", m.Width, m.Height, 0)
	}
	if m.ConfirmDiscard {
		return forms.Fit("Discard unsaved changes? y: discard  n/Esc: keep editing", m.Width, m.Height, 0)
	}
	if m.Help {
		return forms.Fit("CS command library\n\nSearch text matches names, descriptions, tags and literal commands. Selected tags intersect. All clears categories; Untagged finds uncategorized commands.\n\nLibrary: Tab changes pane; arrows select; Enter fills inputs/inserts. Ctrl-L clears filters, Ctrl-R reloads, Ctrl-N creates, Ctrl-E edits, Ctrl-O settings.\n\nInputs: Tab/Shift-Tab navigate, arrows cycle choices. Enter advances then inserts; Ctrl-S submits from any input field; in editor/settings it saves. Repeat values have separate item rows. Esc returns without output.\n\nEditor: Ctrl-Left/Right selects Basics, Inputs, Advanced, Test. Alt-Enter adds a command newline. Ctrl-P marks a precise start/end range and creates an input. Ctrl-S saves only this source. All command placeholders go outside shell quotes. Advanced expressions use inputs.name and pure helpers.\n\nEsc closes help; Ctrl-C/Esc cancel the workspace without executing anything.", m.Width, m.Height, 0)
	}
	switch m.Mode {
	case "recovery":
		return forms.Fit("Library needs repair — no commands can be emitted or saved\n\n"+forms.Safe(strings.Join(m.Library.Diagnostics, "\n"))+"\n\nCtrl-R: retry  Ctrl-O: settings if main YAML parses  Esc: cancel", m.Width, m.Height, 0)
	case "inputs":
		return forms.Fit(m.Form.View(m.Width, m.Height), m.Width, m.Height, 0)
	case "editor":
		return forms.Fit(m.Editor.View(m.Width, m.Height), m.Width, m.Height, 0)
	case "settings":
		return forms.Fit(m.Settings.View(m.Width, m.Height), m.Width, m.Height, 0)
	}
	title := "CS — command library"
	if !m.NoColor {
		title = lipgloss.NewStyle().Bold(true).Render(title)
	}
	header := title + "\nSearch: " + m.Query.OneLine(max(1, m.Width-8), m.Focus == 0) + "\n"
	filter := strings.Join(m.Tags, " + ")
	if m.Untagged {
		filter = "Untagged"
	}
	header += "Filters: " + forms.Fit(forms.Safe(filter), max(1, m.Width-9), 1, 0) + "\n"
	footer := "Tab: pane  Enter: use  Ctrl-N: new  Ctrl-E: edit  Ctrl-O: settings  F1: help  Esc: cancel"
	if m.Width < 70 {
		footer = "Tab:pane Enter:use F1:help Esc:cancel"
	}
	if m.Height < 10 {
		footer = "Tab F1 Esc:cancel"
	}
	footerHeight := strings.Count(forms.Fit(footer, m.Width, m.Height, 0), "\n") + 1
	messageHeight := 1
	if m.Height < 8 {
		messageHeight = 0
	}
	height := max(1, m.Height-3-footerHeight-messageHeight)
	body := ""
	if m.Width >= 110 && m.Height >= 24 {
		left := m.Width / 5
		middle := m.Width * 35 / 100
		right := max(1, m.Width-left-middle-4)
		body = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(left).Render(forms.Fit(m.categories(), left, height, max(0, m.Category-height/2))), "  ", lipgloss.NewStyle().Width(middle).Render(m.results(middle, height)), "  ", forms.Fit(m.detail(), right, height, m.DetailScroll))
	} else if m.Width >= 70 && m.Height >= 16 {
		left := m.Width / 3
		right := max(1, m.Width-left-2)
		content := m.results(right, height)
		if m.ShowDetail || m.Focus == 3 {
			content = forms.Fit(m.detail(), right, height, m.DetailScroll)
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(left).Render(forms.Fit(m.categories(), left, height, max(0, m.Category-height/2))), "  ", content)
	} else {
		pane := []string{"Search / commands", "Categories", "Commands", "Detail"}[m.Focus]
		content := m.results(m.Width, height-1)
		offset := 0
		if m.Focus == 1 {
			content = m.categories()
			offset = max(0, m.Category-max(1, height/2))
		}
		if m.Focus == 3 {
			content = m.detail()
			offset = m.DetailScroll
		}
		body = pane + "\n" + forms.Fit(content, m.Width, max(1, height-1), offset)
	}
	message := m.Message
	if message == "" && len(m.Library.Warnings) > 0 {
		message = strings.Join(m.Library.Warnings, "; ")
	}
	content := header + forms.Fit(body, m.Width, height, 0) + "\n"
	if messageHeight > 0 {
		content += forms.Fit(forms.Safe(message), m.Width, 1, 0) + "\n"
	}
	return content + forms.Fit(footer, m.Width, footerHeight, 0)
}

// Run uses the controlling terminal for input even when command substitution
// redirects stdin/stdout; UI output always goes to stderr.
func Run(lib *library.Library, options Options) (string, error) {
	model, err := New(lib, options)
	if err != nil {
		return "", err
	}
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithInputTTY(), tea.WithOutput(os.Stderr))
	result, err := program.Run()
	if err != nil {
		return "", err
	}
	final := result.(*Model)
	if final.Cancelled {
		return "", nil
	}
	return final.Command, nil
}
