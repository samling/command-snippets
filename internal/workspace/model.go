package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/search"
	forms "github.com/samling/command-snippets/internal/template"
	"github.com/samling/command-snippets/internal/templating"
	"github.com/samling/command-snippets/internal/theme"
)

// Version is the running build, set by the cs command from its ldflags version
// (git describe: last tag, commits since, short hash, -dirty). It is shown on
// the title line, in the editor header and in F1 help.
var Version = "dev"

// titleWithVersion right-aligns a dim version on a title line, dropping the
// version (never the title) when the terminal is too narrow for both.
func titleWithVersion(s *forms.Styles, title string, width int) string {
	plain := forms.Fit(title, width, 1, 0)
	gap := width - ansi.StringWidth(plain) - ansi.StringWidth(Version)
	if Version == "" || gap < 2 {
		return s.Text.Bold(true).Render(plain)
	}
	return s.Text.Bold(true).Render(plain) + strings.Repeat(" ", gap) + s.Label.Render(Version)
}

type Options struct {
	Start   string
	Entry   *library.Entry
	Presets map[string]any
	NoColor bool
}
type Model struct {
	Styles                                  *forms.Styles
	Library                                 *library.Library
	Index                                   *search.Index
	Query                                   forms.Text
	Tags                                    []string
	Untagged                                bool
	Matches                                 []search.Match
	Selected, Category, Focus, DetailScroll int
	// detailScrollMax is the last rendered preview's scroll bound.
	detailScrollMax                                 int
	Width, Height                                   int
	ShowDetail, Help, NoColor                       bool
	NoColorOverride                                 bool
	Mode                                            string
	Form                                            *forms.Form
	Editor                                          *Editor
	Settings                                        *SettingsEditor
	Command, Message                                string
	Cancelled, Done, ConfirmDiscard, DiscardCancels bool
	// DiscardChoice is the highlighted dialog button: 0 Discard, 1 Keep editing.
	DiscardChoice int
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
		m.Editor, err = m.newEditor(nil, m.destination())
		m.Mode = "editor"
	case "edit":
		if lib.Error() != nil {
			return m, nil
		}
		if options.Entry != nil {
			m.Editor, err = m.newEditor(options.Entry, "")
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
	m.applyStyles()
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
	m.Styles = forms.NewStyles(m.Library.Settings, m.NoColor)
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
func (m *Model) canEditSettings() bool {
	source := m.Library.Source(m.Library.ConfigPath)
	return source != nil && source.Root != nil && source.Config.Settings.Validate() == nil
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	defer m.applyStyles()
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.Width = max(0, msg.Width)
		m.Height = max(0, msg.Height)
		return m, nil
	case tea.KeyMsg:
		key := msg.String()
		if m.ConfirmDiscard {
			discard := false
			switch key {
			case "left", "right", "tab", "shift+tab", "h", "l":
				m.DiscardChoice = 1 - m.DiscardChoice
			case "enter", " ":
				discard = m.DiscardChoice == 0
				m.ConfirmDiscard = discard
			case "y", "Y":
				discard = true
			case "n", "N", "esc":
				m.ConfirmDiscard = false
			case "ctrl+c":
				// A second Ctrl-C confirms leaving, as a terminal user expects.
				discard, m.DiscardCancels = true, true
			}
			if discard {
				m.ConfirmDiscard = false
				if m.DiscardCancels {
					return m.exit(true)
				}
				m.back()
			}
			return m, nil
		}
		if key == "f1" && m.Mode == "editor" && !m.Editor.Picker && !m.Help {
			// In the editor F1 is contextual: it toggles help for the focused pane,
			// and works from any field (unlike ?, which can be typed into formulas).
			m.Editor.PaneHelp, m.Editor.HelpScroll = !m.Editor.PaneHelp, 0
			m.Editor.Build()
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
			if key == "esc" && m.Mode == "editor" && (m.Editor.PaneHelp || m.Editor.TagEditing) {
				m.Editor.PaneHelp, m.Editor.TagEditing = false, false
				m.Editor.Build()
				return m, nil
			}
			if key == "esc" && m.Mode == "editor" && m.Editor.CloseInputDetails() {
				return m, nil
			}
			// Esc steps back out of the right pane before it offers to quit.
			if key == "esc" && m.Mode == "editor" && m.Editor.leavePane() {
				return m, nil
			}
			dirty := m.Mode == "editor" && m.Editor.Dirty() || m.Mode == "settings" && m.Settings.Dirty()
			if dirty {
				m.ConfirmDiscard = true
				m.DiscardCancels = key == "ctrl+c"
				m.DiscardChoice = 1 // Keep editing is the safe default
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
				if m.canEditSettings() {
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
			case "ctrl+n", "f2":
				var err error
				m.Editor, err = m.newEditor(nil, m.destination())
				if err != nil {
					m.Message = err.Error()
				} else {
					m.Mode = "editor"
				}
			case "ctrl+e":
				if entry := m.selected(); entry != nil {
					var err error
					m.Editor, err = m.newEditor(entry, "")
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
				switch m.Focus {
				case 3:
					m.scrollDetail(direction)
				case 1:
					m.Category = max(0, min(m.Category+direction, len(m.Index.Categories)+1))
				default:
					m.Selected = max(0, min(m.Selected+direction, max(0, len(m.Matches)-1)))
					m.DetailScroll = 0
				}
			case "pgup", "pgdown":
				if m.Focus == 3 {
					direction := max(1, m.Height/2)
					if key == "pgup" {
						direction = -direction
					}
					m.scrollDetail(direction)
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
				if key == "/" && m.Focus != 0 {
					m.Focus = 0
				} else if m.Focus == 0 {
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
				if key == "?" && (e.PaneHelp || e.helpAllowed()) {
					e.PaneHelp, e.HelpScroll = !e.PaneHelp, 0
					break
				}
				if e.PaneHelp && (key == "up" || key == "down" || key == "pgup" || key == "pgdown") {
					step := map[string]int{"up": -1, "down": 1, "pgup": -10, "pgdown": 10}[key]
					e.HelpScroll = max(0, e.HelpScroll+step)
					break
				}
				e.PaneHelp = false // any other key closes help and acts normally
				if e.tagsFocused() && key == "shift+tab" {
					// Shift-Tab leaves the checklist back to the Tags row itself.
					e.TagEditing = false
					e.Build()
					break
				}
				if e.tagsFocused() && key == "tab" {
					e.TagEditing = false
				}
				if e.tagsFocused() && e.tagKey(key) {
					e.Error = ""
					e.Build()
				} else if e.Section == 3 && e.Test != nil && !e.Picker && e.Focus >= e.TestStart && key != "tab" && key != "shift+tab" && (key != "up" || e.Focus != e.TestStart) {
					e.Test.Focus = e.Focus - e.TestStart
					e.Test.Update(msg) // Test never emits or executes its command.
					e.TestValues = e.Test.Values()
					e.Build()
					// Form.Refresh has already rebuilt dynamic Test controls. Map its
					// focus only after Editor.Build stops interpreting the old slice.
					e.Focus = e.TestStart + e.Test.Focus
				} else {
					switch key {
					case "tab":
						e.step(1)
					case "shift+tab":
						e.step(-1)
					case "up", "down":
						if len(e.Controls) > 0 {
							if e.commandLineMove(key) {
								e.Controls[e.Focus].Update(msg)
							} else if key == "down" {
								e.step(1)
							} else {
								e.step(-1)
							}
						}
					case "enter":
						if len(e.Controls) > 0 {
							if e.Controls[e.Focus].Action != nil {
								e.leaveTest()
								e.Error = ""
								e.Controls[e.Focus].Action()
							} else {
								e.step(1)
							}
						}
					default:
						if len(e.Controls) > 0 {
							if e.tagsFocused() {
								e.beforeTagType(msg)
							}
							if e.Controls[e.Focus].ID == tagsID && !e.TagEditing {
								// Tags only edits after Enter opens its checklist.
							} else if e.Controls[e.Focus].Update(msg) {
								e.leaveTest()
								e.Error = ""
								e.TagMenu = 0
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

// scrollDetail moves the focused preview within its last rendered content.
func (m *Model) scrollDetail(rows int) {
	_ = m.view()
	m.DetailScroll = max(0, min(m.DetailScroll+rows, m.detailScrollMax))
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
func (m *Model) paneTitle(title string, focus int) string {
	if m.Focus == focus {
		return m.Styles.Focused.Bold(true).Render(title)
	}
	return m.Styles.Label.Render(title)
}

func (m *Model) commandRow(name string, selected bool) string {
	if !selected {
		return "  " + forms.Safe(name)
	}
	// A marker remains useful without color; colored rows use background selection.
	if m.Styles.Renderer.ColorProfile() == termenv.Ascii {
		if m.Focus == 2 {
			return "> " + forms.Safe(name)
		}
		return "* " + forms.Safe(name)
	}
	return "  " + forms.Safe(name)
}

func (m *Model) categories(width int) string {
	rows := []string{fmt.Sprintf("All commands (%d)", len(m.Library.Entries)), "Untagged"}
	for _, c := range m.Index.Categories {
		rows = append(rows, fmt.Sprintf("%s (%d)", c.Label, c.Count))
	}
	result := []string{m.paneTitle("Categories · tags", 1), ""}
	for i, row := range rows {
		chosen := i == 0 && len(m.Tags) == 0 && !m.Untagged || i == 1 && m.Untagged
		if i > 1 {
			for _, tag := range m.Tags {
				if tag == m.Index.Categories[i-2].Key {
					chosen = true
				}
			}
		}
		marker := "  "
		if chosen {
			marker = "✓ "
		}
		line := marker + forms.Safe(row)
		style := m.Styles.Text
		if i == m.Category {
			if m.Focus == 1 {
				if m.Styles.Renderer.ColorProfile() == termenv.Ascii {
					line = "> " + line
				} else {
					style = m.Styles.ActiveRow
				}
			} else if chosen {
				style = m.Styles.InactiveRow
			}
		} else if chosen {
			style = m.Styles.Selected
		}
		line = forms.FillRow(style, forms.Fit(line, width, 1, 0), width)
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func (m *Model) results(width, height int) string {
	if height <= 2 && len(m.Matches) > 0 {
		return forms.Fit(m.commandRow(m.Matches[m.Selected].Entry.Snippet.Name, true), width, 1, 0)
	}
	title := m.paneTitle(fmt.Sprintf("Commands (%d)", len(m.Matches)), 2)
	if len(m.Matches) == 0 {
		if len(m.Library.Entries) == 0 {
			return title + "\n\nNo commands yet. F2: create a command."
		}
		return title + "\n\nNo matches. Ctrl-L: clear search and filters."
	}
	counts := map[string]int{}
	for _, match := range m.Matches {
		counts[match.Entry.Snippet.Name]++
	}
	perRow := 3
	if height < 8 {
		perRow = 1
	}
	capacity := max(1, (height-2)/perRow)
	start := max(0, m.Selected-capacity/2)
	start = min(start, max(0, len(m.Matches)-capacity))
	lines := []string{title, ""}
	for i := start; i < min(len(m.Matches), start+capacity); i++ {
		entry := m.Matches[i].Entry
		name := entry.Snippet.Name
		if counts[name] > 1 {
			name += " — " + entry.Source.DisplayPath
		}
		row := forms.Fit(m.commandRow(name, i == m.Selected), width, 1, 0)
		if perRow > 1 {
			tags := strings.Join(entry.Snippet.Tags, " · ")
			if tags == "" {
				tags = "Untagged"
			}
			row += "\n" + m.Styles.Focused.Render(forms.Fit("  "+forms.Safe(tags), width, 1, 0))
		}
		if i == m.Selected && !m.NoColor {
			style := m.Styles.InactiveRow
			if m.Focus == 2 {
				style = m.Styles.ActiveRow
			}
			row = forms.FillRow(style, row, width)
		}
		lines = append(lines, row)
		if perRow > 1 {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) detail(width, height int) string {
	entry := m.selected()
	if entry == nil {
		return m.paneTitle("Preview", 3)
	}
	s := entry.Snippet
	compact := height < 18
	lines := []string{m.paneTitle("Preview", 3), ""}
	previewRows := 4
	if compact {
		lines = []string{m.paneTitle("Command Preview", 3)}
		previewRows = 2
	}
	lines = append(lines, m.Styles.Text.Bold(true).Render(forms.Safe(s.Name)))
	if s.Description != "" {
		lines = append(lines, m.Styles.Label.Render(forms.Safe(s.Description)))
	}
	preview := forms.Fit(forms.Safe(s.Command), max(1, width-4), previewRows, 0)
	box := m.Styles.Panel.Width(max(1, width-2)).Render(m.Styles.Preview.Render(preview))
	if !compact {
		lines = append(lines, "", m.Styles.PreviewTitle.Render("Command Preview"))
	}
	lines = append(lines, box)
	if !compact {
		lines = append(lines, "")
	}
	lines = append(lines, m.Styles.Text.Bold(true).Render("Inputs"))
	for _, in := range s.Inputs {
		state := "optional"
		if in.Required {
			state = "required"
		}
		summary := in.DisplayName() + " · " + state + " · " + in.InputKind()
		if value := fmt.Sprint(in.DefaultValue()); value != "" {
			summary += " · default " + value
		}
		lines = append(lines, forms.Safe(summary))
		if in.Help != "" {
			lines = append(lines, m.Styles.Label.Render(forms.Safe(in.Help)))
		}
	}
	if len(s.Inputs) == 0 {
		lines = append(lines, m.Styles.Label.Render("No inputs · ready to insert"))
	}
	source := m.Styles.Label.Render("Source: " + forms.Safe(entry.Source.DisplayPath))
	if entry.Source.Symlink {
		source += "\nRead-only symlink source"
	}
	source = forms.Fit(source, width, max(1, height/3), 0)
	sourceRows := strings.Count(source, "\n") + 1
	bodyRows := max(1, height-sourceRows-2)
	content := strings.Join(lines, "\n")
	m.detailScrollMax = max(0, strings.Count(ansi.Wrap(content, width, ""), "\n")+1-bodyRows)
	body := forms.Fit(content, width, bodyRows, m.DetailScroll)
	// Keep source attribution visible while the preview/input summary scrolls.
	return body + "\n" + m.Styles.Label.Render(strings.Repeat("─", max(1, width))) + "\n" + source
}

func (m *Model) framedPane(content string, width, height, focus, offset int) string {
	style := m.Styles.Panel
	if m.Focus == focus {
		style = style.BorderForeground(lipgloss.Color(m.Styles.Colors[theme.Focus]))
	}
	return style.Width(max(1, width-2)).Height(max(1, height-2)).Render(forms.Fit(content, max(1, width-4), max(1, height-2), offset))
}

func (m *Model) View() string {
	m.applyStyles()
	return m.Styles.Surface(m.view(), m.Width, m.Height)
}
func (m *Model) applyStyles() {
	if m.Form != nil {
		m.Form.Styles = m.Styles
	}
	if m.Editor != nil {
		m.Editor.Styles = m.Styles
		if m.Editor.Test != nil {
			m.Editor.Test.Styles = m.Styles
		}
	}
	if m.Settings != nil {
		m.Settings.Styles = m.Styles
	}
}
func (m *Model) view() string {
	if m.Done {
		return ""
	}
	if m.Width < 24 || m.Height < 6 {
		return forms.Fit("Resize terminal or Esc to cancel.", m.Width, m.Height, 0)
	}
	if m.ConfirmDiscard {
		return m.discardDialog()
	}
	if m.Help {
		return forms.Fit("CS command library · "+Version+"\n\nSearch text matches names, descriptions, tags and literal commands. Selected tags intersect. All clears categories; Untagged finds uncategorized commands.\n\nLibrary: Tab changes pane; arrows select; Enter fills inputs/inserts. Ctrl-L clears filters, Ctrl-R reloads, F2 creates, Ctrl-E edits, Ctrl-O settings. Up/Down and PgUp/PgDn scroll the focused preview; Ctrl-D toggles detail at narrow widths.\n\nInputs: Tab/Shift-Tab navigate, arrows cycle choices. Enter advances then inserts; Ctrl-S submits from any input field; in editor/settings it saves. Repeat values have separate item rows. Esc returns without output. Regex: Ctrl-R toggles the focused explanation pane; Ctrl-U/D or PgUp/PgDn scrolls it.\n\nEditor: Tab/Shift-Tab or Up/Down move between controls; in a multiline Command, Up/Down move between its lines first. Enter on Configure inputs, Expressions or Test preview opens that pane and focuses it. Inputs lists every input; Enter opens its details, and Esc or Back to inputs returns to the list. Type {{name}} to create a required text input automatically. Tags are whitespace-separated. Alt-Enter adds a command newline. Inline actions open expressions and Test preview. Ctrl-S saves only this source. All command placeholders go outside shell quotes. Advanced expressions use inputs.name and pure helpers.\n\nEsc closes help; Ctrl-C/Esc cancel the workspace without executing anything.", m.Width, m.Height, 0)
	}
	switch m.Mode {
	case "recovery":
		footer := "Ctrl-R: retry"
		if m.canEditSettings() {
			footer += "  Ctrl-O: settings"
		}
		footer += "  Esc: cancel"
		footer = forms.Fit(footer, m.Width, m.Height-1, 0)
		footerRows := strings.Count(footer, "\n") + 1
		body := "Library needs repair — no commands can be emitted or saved\n\n" + m.Styles.Error.Render(forms.Safe(strings.Join(m.Library.Diagnostics, "\n")))
		return forms.Fit(body, m.Width, max(1, m.Height-footerRows-1), 0) + "\n" + footer
	case "inputs":
		return forms.Fit(m.Form.View(m.Width, m.Height), m.Width, m.Height, 0)
	case "editor":
		return forms.Fit(m.Editor.View(m.Width, m.Height), m.Width, m.Height, 0)
	case "settings":
		return forms.Fit(m.Settings.View(m.Width, m.Height), m.Width, m.Height, 0)
	}
	title := titleWithVersion(m.Styles, "CS — command library", m.Width)
	filterLabels := []string{}
	for _, tag := range m.Tags {
		for _, category := range m.Index.Categories {
			if tag == category.Key {
				filterLabels = append(filterLabels, category.Label)
			}
		}
	}
	filter := strings.Join(filterLabels, " + ")
	if m.Untagged {
		filter = "Untagged"
	}
	footer := "Enter: use  F2: new  Ctrl-E: edit  Ctrl-O: settings  Tab: pane  F1: help  Esc: cancel"
	if m.Focus == 3 {
		footer += "  ↑↓/PgUp/PgDn: scroll"
	}
	if m.Width < 70 {
		footer = "F2:new Tab:pane Enter:use F1:help Esc:cancel"
	}
	if m.Height < 10 {
		footer = "F2:new Tab F1 Esc:cancel"
	}
	footerHeight := strings.Count(forms.Fit(footer, m.Width, m.Height, 0), "\n") + 1
	message := m.Message
	if message == "" && len(m.Library.Warnings) > 0 {
		message = strings.Join(m.Library.Warnings, "; ")
	}
	// Failure reasons take precedence over list content in compact terminals.
	if m.Height < 12 {
		status := ""
		statusRows := 0
		if message != "" {
			status = forms.Fit(forms.Safe(message), m.Width, max(1, m.Height-footerHeight-1), 0)
			statusRows = strings.Count(status, "\n") + 1
		}
		header := title + "\n" + m.paneTitle("Search", 0) + ": " + m.Query.OneLineWithRenderer(max(1, m.Width-10), m.Focus == 0, m.Styles.Renderer)
		if filter != "" {
			header += "\nTags: " + forms.Safe(filter)
		}
		header = forms.Fit(header, m.Width, max(1, m.Height-footerHeight-statusRows), 0)
		headerRows := strings.Count(header, "\n") + 1
		height := max(0, m.Height-headerRows-footerHeight-statusRows)
		parts := []string{header}
		if height > 0 {
			content := m.results(m.Width, height)
			switch m.Focus {
			case 1:
				content = forms.Fit(m.categories(m.Width), m.Width, height, max(0, m.Category+2-height/2))
			case 3:
				content = forms.Fit(m.detail(m.Width, height), m.Width, height, 0)
			}
			parts = append(parts, forms.Fit(content, m.Width, height, 0))
		}
		if status != "" {
			parts = append(parts, status)
		}
		parts = append(parts, forms.Fit(footer, m.Width, footerHeight, 0))
		return forms.Fit(strings.Join(parts, "\n"), m.Width, m.Height, 0)
	}
	searchLabel := "Search commands: "
	searchText := m.Query.OneLineWithRenderer(max(1, m.Width-4-len(searchLabel)), m.Focus == 0, m.Styles.Renderer)
	searchRows := 1
	if filter != "" {
		metadata := "  · tag: " + forms.Safe(filter)
		if lipgloss.Width(searchLabel+searchText+metadata) > m.Width-4 {
			searchText += "\nTags: " + forms.Safe(filter)
			searchRows = 2
		} else {
			searchText += metadata
		}
	}
	searchStyle := m.Styles.Panel
	if m.Focus == 0 {
		searchStyle = searchStyle.BorderForeground(lipgloss.Color(m.Styles.Colors[theme.Focus]))
	}
	searchBox := searchStyle.Width(m.Width - 2).Render(forms.Fit(searchLabel+searchText, m.Width-4, searchRows, 0))
	header := title + "\n" + m.Styles.Label.Render(strings.Repeat("─", m.Width)) + "\n" + searchBox + "\n\n"
	headerRows := strings.Count(header, "\n")
	// Reserve a separated footer and an optional one-line status.
	statusRows := 0
	if message != "" {
		statusRows = 1
	}
	height := max(3, m.Height-headerRows-footerHeight-1-statusRows)
	body := ""
	if m.Width >= 110 && m.Height >= 20 {
		left := m.Width / 5
		middle := m.Width / 3
		right := m.Width - left - middle
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.framedPane(m.categories(left-4), left, height, 1, max(0, m.Category-(height-2)/2)),
			m.framedPane(m.results(middle-4, height-2), middle, height, 2, 0),
			m.framedPane(m.detail(right-4, height-2), right, height, 3, 0))
	} else if m.Width >= 70 && m.Height >= 16 {
		left := m.Width / 3
		right := m.Width - left
		content := m.results(right-4, height-2)
		focus := 2
		offset := 0
		if m.ShowDetail || m.Focus == 3 {
			content = m.detail(right-4, height-2)
			focus = 3
			offset = 0
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.framedPane(m.categories(left-4), left, height, 1, max(0, m.Category-(height-2)/2)), m.framedPane(content, right, height, focus, offset))
	} else {
		content := m.results(m.Width-4, height-2)
		focus := 2
		offset := 0
		if m.Focus == 1 {
			content = m.categories(m.Width - 4)
			focus = 1
			offset = max(0, m.Category-(height-2)/2)
		} else if m.Focus == 3 || m.ShowDetail {
			content = m.detail(m.Width-4, height-2)
			focus = 3
			offset = 0
		}
		body = m.framedPane(content, m.Width, height, focus, offset)
	}
	content := header + body + "\n"
	if message != "" {
		content += forms.Fit(forms.Safe(message), m.Width, 1, 0) + "\n"
	}
	content += m.Styles.Label.Render(strings.Repeat("─", m.Width)) + "\n" + m.Styles.Help.Render(forms.Fit(footer, m.Width, footerHeight, 0))
	return forms.Fit(content, m.Width, m.Height, 0)
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

// newEditor opens the editor with the library's tags available to its Tags menu.
func (m *Model) newEditor(entry *library.Entry, destination string) (*Editor, error) {
	e, err := NewEditor(entry, destination)
	if e != nil {
		e.KnownTags = append(e.KnownTags, m.Index.Categories...)
	}
	return e, err
}

// discardDialog draws a framed confirmation centred over the dimmed screen it
// would discard. Keep editing is the default; the Discard button is tinted with
// the error role so the destructive choice is never the easy one to misread.
func (m *Model) discardDialog() string {
	s := m.Styles
	m.ConfirmDiscard = false
	behind := m.view()
	m.ConfirmDiscard = true
	what := "Your changes have not been saved."
	switch {
	case m.Mode == "editor" && m.Editor.Name.String() != "":
		what = "Your edits to “" + forms.Safe(strings.TrimSpace(m.Editor.Name.String())) + "” have not been saved."
	case m.Mode == "settings":
		what = "Your settings changes have not been saved."
	}
	width := min(m.Width-2, 46)
	if width < 20 {
		return forms.Fit(s.Error.Render("Discard changes?")+"\n"+s.Label.Render("y: discard  n/Esc: keep"), m.Width, m.Height, 0)
	}
	inner := width - 4
	plain := s.Renderer.ColorProfile() == termenv.Ascii
	button := func(label string, selected, danger bool) string {
		text := " " + label + " "
		if plain {
			// Without colour, brackets mark the selected button.
			if selected {
				return "[" + label + "]"
			}
			return " " + label + " "
		}
		switch {
		case selected && danger:
			return s.Renderer.NewStyle().Reverse(true).Inherit(s.Error).Render(text)
		case selected:
			return s.ActiveRow.Bold(true).Render(text)
		case danger:
			return " " + s.Error.Render(label) + " "
		}
		return " " + s.Text.Render(label) + " "
	}
	buttons := button("Discard", m.DiscardChoice == 0, true) + "   " + button("Keep editing", m.DiscardChoice == 1, false)
	if ansi.StringWidth(buttons) > inner {
		buttons = button("Discard", m.DiscardChoice == 0, true) + " " + button("Keep", m.DiscardChoice == 1, false)
	}
	hint := "←→ choose · Enter confirm · y / n"
	if m.Height < 12 {
		hint = "y discard · n keep"
	}
	body := s.Text.Render(forms.Fit(what, inner, 3, 0))
	lines := []string{}
	if m.Height >= 12 {
		lines = append(lines, "", body, "")
	} else {
		lines = append(lines, body)
	}
	lines = append(lines, buttons)
	if m.Height >= 10 {
		lines = append(lines, "", s.Label.Render(forms.Fit(hint, inner, 1, 0)))
	}
	border := s.Renderer.NewStyle().Foreground(lipgloss.Color(s.Colors[theme.Error]))
	// The title sits in the top border: ╭─ Discard unsaved changes? ───╮
	title := forms.Fit("Discard unsaved changes?", width-6, 1, 0)
	top := border.Render("╭─ ") + s.Error.Bold(true).Render(title) + border.Render(" "+strings.Repeat("─", max(0, width-5-ansi.StringWidth(title)))+"╮")
	rest := s.Renderer.NewStyle().Border(lipgloss.RoundedBorder()).BorderTop(false).BorderForeground(lipgloss.Color(s.Colors[theme.Error])).Padding(0, 1).Width(width - 2).Render(strings.Join(lines, "\n"))
	return overlayCenter(dim(s, behind), top+"\n"+rest, m.Width, m.Height)
}

// dim renders a screen in the muted role so a dialog over it stands out.
func dim(s *forms.Styles, view string) string {
	lines := strings.Split(ansi.Strip(view), "\n")
	for i, line := range lines {
		lines[i] = s.Label.Faint(true).Render(line)
	}
	return strings.Join(lines, "\n")
}

// overlayCenter places box over the middle of a width x height screen.
func overlayCenter(screen, box string, width, height int) string {
	lines := strings.Split(forms.Fit(screen, width, height, 0), "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	boxLines := strings.Split(box, "\n")
	boxWidth := 0
	for _, line := range boxLines {
		boxWidth = max(boxWidth, ansi.StringWidth(line))
	}
	top := max(0, (height-len(boxLines))/2)
	left := max(0, (width-boxWidth)/2)
	for i, line := range boxLines {
		row := top + i
		if row >= len(lines) {
			break
		}
		base := lines[row]
		prefix := ansi.Truncate(base, left, "")
		prefix += strings.Repeat(" ", max(0, left-ansi.StringWidth(prefix)))
		suffix := ansi.TruncateLeft(base, left+boxWidth, "")
		lines[row] = prefix + line + strings.Repeat(" ", max(0, boxWidth-ansi.StringWidth(line))) + suffix
	}
	return strings.Join(lines, "\n")
}
