package template

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/theme"
	"golang.org/x/term"
)

// Shared muted palette; focus is teal, while errors and missing values remain distinct.
var (
	TextStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#e3e8ef"))
	FocusedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#82d4c4"))
	LabelStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#a8b3c3"))
	SelectedStyle     = FocusedStyle.Bold(true)
	UnselectedStyle   = TextStyle
	ErrorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#d3a6a6"))
	HelpStyle         = LabelStyle
	PreviewStyle      = TextStyle
	PreviewTitleStyle = FocusedStyle.Bold(true)
	FilledStyle       = FocusedStyle
	UnfilledStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#c3b896"))
	BorderColor       = lipgloss.Color("#394353")
	PanelStyle        = TextStyle.Border(lipgloss.RoundedBorder()).BorderForeground(BorderColor).Padding(0, 1)
	ActiveRowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ecfffb")).Background(lipgloss.Color("#30484a"))
	InactiveRowStyle  = TextStyle.Background(lipgloss.Color("#252e38"))
)

// Control palettes are explicit so the original input form cannot recolor the library/editor.
type controlPalette struct {
	Focused, Label, Selected, Unselected, Error, Value lipgloss.Style
	Renderer                                           *lipgloss.Renderer
	Compact, HighlightRow                              bool
	Row                                                lipgloss.Style
	// Wells give each value a filled input box; the focused row gets an accent
	// bar. Both are colour-only, so plain terminals keep the "> " marker.
	Wells           bool
	Well, FocusWell lipgloss.Style
	Accent          lipgloss.Style
}

var (
	mutedControlPalette = controlPalette{
		Focused: FocusedStyle, Label: LabelStyle, Selected: SelectedStyle,
		Unselected: UnselectedStyle, Error: ErrorStyle,
	}
)

// Styles belongs to a single workspace/form. Neither its palette nor its output
// profile changes lipgloss's global renderer or another model.
type Styles struct {
	Renderer *lipgloss.Renderer
	Colors   theme.Palette
	// Named themes and explicit text overrides own the foreground, even on transparent surfaces.
	paintText                                                                                                bool
	Text, Focused, Label, Selected, Unselected, Error, Help                                                  lipgloss.Style
	Preview, PreviewTitle, Filled, Unfilled, Panel, ActiveRow, InactiveRow                                   lipgloss.Style
	InputPreview, InputPreviewTitle, InputFilled, InputUnfilled, InputHelp, InputRegexPanel, InputRegexTitle lipgloss.Style
	Controls, InputControls                                                                                  controlPalette
}

func NewStyles(settings models.Settings, noColor bool) *Styles {
	r := lipgloss.NewRenderer(os.Stderr)
	profile := termenv.Ascii
	if !noColor && os.Getenv("NO_COLOR") == "" && settings.Color != "never" {
		if settings.Color == "always" {
			profile = termenv.TrueColor
		} else if term.IsTerminal(int(os.Stderr.Fd())) {
			profile = termenv.NewOutput(os.Stderr).Profile
		}
	}
	r.SetColorProfile(profile)
	return stylesFor(r, settings)
}

func stylesFor(r *lipgloss.Renderer, settings models.Settings) *Styles {
	colors := theme.Resolve(settings.Theme, settings.ThemeColors)
	fg := func(role theme.Role) lipgloss.Style { return r.NewStyle().Foreground(lipgloss.Color(colors[role])) }
	background := func(s lipgloss.Style, role theme.Role) lipgloss.Style {
		if colors[role] != "" {
			s = s.Background(lipgloss.Color(colors[role]))
		}
		return s
	}
	s := &Styles{Renderer: r, Colors: colors, paintText: settings.Theme == "catppuccin-mocha" || settings.ThemeColors[string(theme.Text)] != ""}
	s.Text = fg(theme.Text)
	s.Focused = fg(theme.Focus)
	s.Label = fg(theme.Muted)
	s.Selected = s.Focused.Bold(true)
	if settings.Theme == "catppuccin-mocha" || settings.ThemeColors[string(theme.Selection)] != "" {
		s.Selected = s.Selected.Background(lipgloss.Color(colors[theme.Selection]))
	}
	s.Unselected = s.Text
	s.Error = fg(theme.Error)
	s.Help = s.Label
	s.Preview = fg(theme.Preview)
	s.PreviewTitle = s.Focused.Bold(true)
	s.Filled = fg(theme.Filled)
	s.Unfilled = fg(theme.Unfilled)
	s.Panel = background(s.Text, theme.Panel).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colors[theme.Border])).Padding(0, 1)
	s.ActiveRow = s.Text.Background(lipgloss.Color(colors[theme.Selection]))
	if (settings.Theme == "" || settings.Theme == "default") && !s.paintText {
		s.ActiveRow = s.ActiveRow.Foreground(lipgloss.Color("#ecfffb"))
	}
	s.InactiveRow = s.Text.Background(lipgloss.Color(colors[theme.InactiveSelection]))
	s.Controls = controlPalette{Focused: s.Focused, Label: s.Label, Selected: s.Selected, Unselected: s.Unselected, Error: s.Error, Renderer: r}
	if s.paintText {
		s.Controls.Value = s.Text
	}
	s.Controls.Compact = true
	s.Controls.HighlightRow = true
	s.Controls.Row = s.ActiveRow
	s.InputControls = s.Controls
	s.InputControls.Compact = false
	s.InputControls.HighlightRow = false
	s.InputPreview = s.Preview
	s.InputPreviewTitle = s.Preview.Bold(true)
	s.InputFilled = s.Filled
	s.InputUnfilled = s.Unfilled.Bold(true)
	s.InputHelp = s.Help
	s.InputRegexPanel = s.Panel
	s.InputRegexTitle = s.Focused.Bold(true)
	defer func() {
		// The command form's input wells use the theme's selection surfaces.
		if r.ColorProfile() != termenv.Ascii && colors[theme.Selection] != "" && colors[theme.InactiveSelection] != "" {
			s.InputControls.Wells = true
			s.InputControls.Well = r.NewStyle().Background(lipgloss.Color(colors[theme.InactiveSelection]))
			s.InputControls.FocusWell = r.NewStyle().Background(lipgloss.Color(colors[theme.Selection]))
			s.InputControls.Accent = s.InputControls.Focused
		}
	}()
	if settings.Theme == "" || settings.Theme == "default" {
		indexed := func(value string) lipgloss.Style { return r.NewStyle().Foreground(lipgloss.Color(value)) }
		s.InputControls = controlPalette{Focused: indexed("205"), Label: indexed("241"), Selected: indexed("86").Bold(true), Unselected: indexed("247"), Error: indexed("196"), Renderer: r}
		s.InputPreview = indexed("86")
		s.InputPreviewTitle = s.InputPreview.Bold(true)
		s.InputFilled = indexed("120")
		s.InputUnfilled = indexed("208").Bold(true)
		s.InputHelp = indexed("241")
		s.InputRegexPanel = indexed("245").Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
		s.InputRegexTitle = indexed("214").Bold(true)
		// Overrides apply to the original mixed default appearance too.
		for role := range settings.ThemeColors {
			switch theme.Role(role) {
			case theme.Focus:
				s.InputControls.Focused = s.Focused
				s.InputControls.Selected = s.Selected
				s.InputRegexTitle = s.Focused.Bold(true)
			case theme.Text:
				s.InputControls.Unselected = s.Text
				s.InputControls.Value = s.Text
			case theme.Muted:
				s.InputControls.Label = s.Label
				s.InputHelp = s.Help
			case theme.Error:
				s.InputControls.Error = s.Error
			case theme.Preview:
				s.InputPreview = s.Preview
				s.InputPreviewTitle = s.Preview.Bold(true)
			case theme.Selection:
				s.InputControls.Selected = s.InputControls.Selected.Background(lipgloss.Color(colors[theme.Selection]))
			case theme.Filled:
				s.InputFilled = s.Filled
			case theme.Unfilled:
				s.InputUnfilled = s.Unfilled.Bold(true)
			case theme.Border, theme.Panel:
				s.InputRegexPanel = s.Panel
			}
		}
	}
	return s
}
func (s *Styles) ControlsView(controls []Control, focus, width, height int) string {
	return controlsView(controls, focus, width, height, s.Controls)
}

// AlignedControlsView renders editor controls as label/value columns with the
// focused control's help and error in a hint at the bottom; focus < 0 omits it.
func (s *Styles) AlignedControlsView(controls []Control, focus, width, height int) string {
	view, _ := alignedView(controls, focus, width, height, s.Controls)
	return view
}

const sgrReset = "\x1b[0m"

// continuous re-applies style's colors after every inner SGR reset. lipgloss
// opens a style once per line, so a styled span inside it would otherwise end
// the row/panel background at its reset. The Ascii profile has no sequence.
func continuous(style lipgloss.Style, content string) string {
	marked := style.Inline(true).UnsetWidth().UnsetHeight().UnsetMaxWidth().UnsetMaxHeight().Render("x")
	prefix, _, _ := strings.Cut(marked, "x")
	if prefix == "" {
		return content
	}
	return strings.ReplaceAll(content, sgrReset, sgrReset+prefix)
}

// FillRow paints a highlighted row's background across its styled spans and
// pads every line to width. Foregrounds and the reverse cursor inside remain.
func FillRow(style lipgloss.Style, content string, width int) string {
	return style.Width(width).Render(continuous(style, content))
}
func (s *Styles) Surface(view string, width, height int) string {
	if view == "" || (s.Colors[theme.Background] == "" && !s.paintText) {
		return Fit(view, width, height, 0)
	}
	if s.Colors[theme.Background] == "" {
		return Fit(s.Text.Render(Fit(view, width, height, 0)), width, height, 0)
	}
	style := s.Text.Width(width).Height(height)
	if color := s.Colors[theme.Background]; color != "" {
		style = style.Background(lipgloss.Color(color))
	}
	return Fit(style.Render(Fit(view, width, height, 0)), width, height, 0)
}
func (s *Styles) Frame(content string, width, height int, focused bool) string {
	style := s.Panel
	if focused {
		style = style.BorderForeground(lipgloss.Color(s.Colors[theme.Focus]))
	}
	content = Fit(content, max(1, width-4), max(1, height-2), 0)
	if color := s.Colors[theme.Panel]; color != "" {
		// Opaque panel fill continues after styled spans; foregrounds stay as rendered.
		content = continuous(s.Renderer.NewStyle().Background(lipgloss.Color(color)), content)
	}
	return style.Width(max(1, width-2)).Height(max(1, height-2)).Render(content)
}

// DefaultStyles keeps the existing public form/control entry points on the default renderer.
func DefaultStyles() *Styles { return stylesFor(lipgloss.DefaultRenderer(), models.Settings{}) }
