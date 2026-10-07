package models

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/samling/command-snippets/internal/theme"
	"golang.org/x/text/cases"
)

// Config is the human-editable document; included sources own snippets, not settings.
type Config struct {
	Settings Settings  `yaml:"settings,omitempty"`
	Snippets []Snippet `yaml:"snippets"`
}

type Settings struct {
	Sources       []string          `yaml:"sources,omitempty"`
	ProjectSource bool              `yaml:"project_source"`
	DefaultSource string            `yaml:"default_source,omitempty"`
	Color         string            `yaml:"color,omitempty"`
	Theme         string            `yaml:"theme,omitempty"`
	ThemeColors   map[string]string `yaml:"theme_colors,omitempty"`
}

func DefaultSettings() Settings { return Settings{ProjectSource: true, Color: "auto"} }
func (s Settings) Validate() error {
	if err := theme.Validate(s.Theme, s.ThemeColors); err != nil {
		return err
	}
	if s.Color != "auto" && s.Color != "always" && s.Color != "never" {
		return fmt.Errorf("color must be auto, always, or never")
	}
	for _, p := range s.Sources {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("source paths cannot be empty")
		}
	}
	return nil
}

type Snippet struct {
	ID          string            `yaml:"id,omitempty" json:"id,omitempty"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Tags        []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	Command     string            `yaml:"command" json:"command"`
	Inputs      []Input           `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Expressions map[string]string `yaml:"expressions,omitempty" json:"expressions,omitempty"`
}

type Input struct {
	Name         string            `yaml:"name"`
	Label        string            `yaml:"label,omitempty"`
	Help         string            `yaml:"help,omitempty"`
	Kind         string            `yaml:"kind,omitempty"`
	Default      any               `yaml:"default,omitempty"`
	Required     bool              `yaml:"required,omitempty"`
	Flag         string            `yaml:"flag,omitempty"`
	Choices      []Choice          `yaml:"choices,omitempty"`
	Special      map[string]string `yaml:"special,omitempty"`
	Validate     *Validation       `yaml:"validate,omitempty"`
	VisibleWhen  *Condition        `yaml:"visible_when,omitempty"`
	RequiredWhen *Condition        `yaml:"required_when,omitempty"`
}

type Choice struct {
	Label string `yaml:"label"`
	Value string `yaml:"value"`
}
type Validation struct {
	Pattern string `yaml:"pattern,omitempty"`
	Range   []int  `yaml:"range,omitempty"`
	Regex   bool   `yaml:"regex,omitempty"`
}
type Condition struct {
	Input     string `yaml:"input,omitempty"`
	Equals    any    `yaml:"equals,omitempty"`
	NotEquals any    `yaml:"not_equals,omitempty"`
	Expr      string `yaml:"expr,omitempty"`
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ValidIdentifier(s string) bool { return identifier.MatchString(s) }
func ValidID(s string) bool         { return uuid.MatchString(s) }
func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
func TagKey(s string) string { return cases.Fold().String(strings.TrimSpace(s)) }
func (in Input) InputKind() string {
	if in.Kind == "" {
		return "text"
	}
	return in.Kind
}
func (in Input) DisplayName() string {
	if in.Label != "" {
		return in.Label
	}
	return in.Name
}
func (in Input) Zero() any {
	switch in.InputKind() {
	case "toggle":
		return false
	case "repeat":
		return []string{}
	default:
		return ""
	}
}
func (in Input) DefaultValue() any {
	if in.Default != nil {
		if in.InputKind() == "repeat" {
			switch v := in.Default.(type) {
			case []any:
				r := make([]string, len(v))
				for i, x := range v {
					r[i], _ = x.(string)
				}
				return r
			case []string:
				return append([]string{}, v...)
			}
		}
		return in.Default
	}
	if in.InputKind() == "choice" && len(in.Choices) > 0 {
		return in.Choices[0].Label
	}
	return in.Zero()
}
func noControls(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (s *Snippet) Validate() error {
	if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Command) == "" {
		return fmt.Errorf("name and command are required; example: snippets: [{name: List pods, command: kubectl get pods}]")
	}
	if !noControls(s.Name) {
		return fmt.Errorf("name must not contain terminal control characters")
	}
	s.Name = strings.TrimSpace(s.Name)
	if s.ID != "" && !ValidID(s.ID) {
		return fmt.Errorf("id must be a canonical lowercase UUID v4")
	}
	seenTags := map[string]bool{}
	tags := []string{}
	for _, tag := range s.Tags {
		if !noControls(tag) {
			return fmt.Errorf("tags must not contain terminal control characters")
		}
		tag = strings.TrimSpace(tag)
		if tag == "" || !noControls(tag) {
			return fmt.Errorf("tags must be nonempty and contain no control characters")
		}
		key := TagKey(tag)
		if !seenTags[key] {
			seenTags[key] = true
			tags = append(tags, tag)
		}
	}
	s.Tags = tags
	names := map[string]bool{}
	for _, in := range s.Inputs {
		if !ValidIdentifier(in.Name) || names[in.Name] {
			return fmt.Errorf("input %q must have a unique identifier", in.Name)
		}
		names[in.Name] = true
		kind := in.InputKind()
		switch kind {
		case "text", "flag", "toggle", "choice", "repeat":
		default:
			return fmt.Errorf("input %s: unknown kind %q", in.Name, kind)
		}
		if (kind == "flag" || kind == "toggle" || kind == "repeat") && strings.TrimSpace(in.Flag) == "" {
			return fmt.Errorf("input %s requires a flag", in.Name)
		}
		if in.Flag != "" && kind != "flag" && kind != "toggle" && kind != "repeat" {
			return fmt.Errorf("input %s: flag is only valid for flag, toggle, repeat", in.Name)
		}
		if len(in.Special) > 0 && (kind == "toggle" || kind == "repeat") {
			return fmt.Errorf("input %s: special output is not supported for %s", in.Name, kind)
		}
		if kind == "choice" {
			if len(in.Choices) == 0 {
				return fmt.Errorf("input %s requires choices", in.Name)
			}
			labels := map[string]bool{}
			for _, c := range in.Choices {
				if strings.TrimSpace(c.Label) == "" || labels[c.Label] {
					return fmt.Errorf("input %s: choice labels must be nonempty and unique", in.Name)
				}
				labels[c.Label] = true
			}
		}
		if kind != "choice" && len(in.Choices) > 0 {
			return fmt.Errorf("input %s: choices require kind choice", in.Name)
		}
		if in.Default != nil {
			switch kind {
			case "toggle":
				if _, ok := in.Default.(bool); !ok {
					return fmt.Errorf("input %s: default must be boolean", in.Name)
				}
			case "repeat":
				switch d := in.Default.(type) {
				case []string:
				case []any:
					for _, v := range d {
						if _, ok := v.(string); !ok {
							return fmt.Errorf("input %s: default items must be strings", in.Name)
						}
					}
				default:
					return fmt.Errorf("input %s: default must be a string list", in.Name)
				}
			default:
				if _, ok := in.Default.(string); !ok {
					return fmt.Errorf("input %s: default must be a string", in.Name)
				}
			}
		}
		if in.Validate != nil {
			if kind == "toggle" {
				return fmt.Errorf("input %s: toggle inputs do not support pattern/range/regex content validation; use required or required_when", in.Name)
			}
			v := in.Validate
			if v.Pattern != "" {
				if _, err := regexp.Compile(v.Pattern); err != nil {
					return fmt.Errorf("input %s: pattern: %w", in.Name, err)
				}
			}
			if v.Range != nil && (len(v.Range) != 2 || v.Range[0] > v.Range[1]) {
				return fmt.Errorf("input %s: range must contain two ordered bounds", in.Name)
			}
		}
	}
	for name := range s.Expressions {
		if !ValidIdentifier(name) || names[name] {
			return fmt.Errorf("expression %q must have a unique identifier", name)
		}
		names[name] = true
	}
	for _, in := range s.Inputs {
		for _, c := range []*Condition{in.VisibleWhen, in.RequiredWhen} {
			if c == nil {
				continue
			}
			if c.Expr != "" {
				if c.Input != "" || c.Equals != nil || c.NotEquals != nil {
					return fmt.Errorf("input %s: condition must use expr or a comparison, not both", in.Name)
				}
			} else {
				if c.Input == in.Name || !names[c.Input] {
					return fmt.Errorf("input %s: invalid condition input %q", in.Name, c.Input)
				}
				if (c.Equals != nil) == (c.NotEquals != nil) {
					return fmt.Errorf("input %s: condition needs exactly one of equals or not_equals", in.Name)
				}
			}
		}
	}
	return nil
}
