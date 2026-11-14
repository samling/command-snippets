// Package theme defines the small semantic color contract shared by all screens.
package theme

import (
	"fmt"
	"regexp"
	"sort"
)

type Role string

const (
	Text              Role = "text"
	Muted             Role = "muted"
	Focus             Role = "focus"
	Preview           Role = "preview"
	Filled            Role = "filled"
	Unfilled          Role = "unfilled"
	Error             Role = "error"
	Border            Role = "border"
	Selection         Role = "selection"
	InactiveSelection Role = "inactive_selection"
	Background        Role = "background"
	Panel             Role = "panel"
)

type Palette map[Role]string

var defaults = Palette{
	Text: "#e3e8ef", Muted: "#a8b3c3", Focus: "#82d4c4", Preview: "#e3e8ef",
	Filled: "#82d4c4", Unfilled: "#c3b896", Error: "#d3a6a6", Border: "#394353",
	Selection: "#30484a", InactiveSelection: "#252e38", Background: "", Panel: "",
}

// Mocha values come from https://catppuccin.com/palette.
var mocha = Palette{
	Text: "#cdd6f4", Muted: "#a6adc8", Focus: "#cba6f7", Preview: "#89b4fa",
	Filled: "#a6e3a1", Unfilled: "#fab387", Error: "#f38ba8", Border: "#585b70",
	Selection: "#45475a", InactiveSelection: "#313244", Background: "#1e1e2e", Panel: "#181825",
}
var hex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func Validate(name string, overrides map[string]string) error {
	if name != "" && name != "default" && name != "catppuccin-mocha" {
		return fmt.Errorf("unknown theme %q; choose default or catppuccin-mocha", name)
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := defaults[Role(key)]; !ok {
			return fmt.Errorf("unknown theme color role %q", key)
		}
		surface := Role(key) == Background || Role(key) == Panel
		if surface && overrides[key] == "transparent" {
			continue
		}
		if !hex.MatchString(overrides[key]) {
			if surface {
				return fmt.Errorf("theme color %s must be #RRGGBB or transparent", key)
			}
			return fmt.Errorf("theme color %s must be #RRGGBB", key)
		}
	}
	return nil
}
func Resolve(name string, overrides map[string]string) Palette {
	base := defaults
	if name == "catppuccin-mocha" {
		base = mocha
	}
	palette := make(Palette, len(base))
	for role, value := range base {
		palette[role] = value
	}
	// Invalid settings still need a safe recovery screen.
	if Validate(name, overrides) == nil {
		for role, value := range overrides {
			if value == "transparent" {
				value = ""
			}
			palette[Role(role)] = value
		}
	}
	return palette
}
