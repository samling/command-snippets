package models

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"
)

// SnippetSource represents where a snippet was loaded from
type SnippetSource string

const (
	SourceGlobal SnippetSource = "global"
	SourceLocal  SnippetSource = "local"
)

// Built-in variable type identifiers. User-defined types in
// Config.VariableTypes use arbitrary strings; these are the two the engine
// treats specially.
const (
	VarTypeBoolean = "boolean"
	VarTypeRegex   = "regex"
)

// parseBool returns true for the truthy string forms accepted by snippet
// boolean variables. Anything else is false (including the empty string).
func parseBool(s string) bool {
	switch s {
	case "true", "yes", "1":
		return true
	}
	return false
}

// placeholderPattern matches <name> tokens in command templates. Variable
// names are letters/digits/underscores starting with a letter or underscore.
var placeholderPattern = regexp.MustCompile(`<([A-Za-z_][A-Za-z0-9_]*)>`)

// Snippet represents a command template
type Snippet struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Command     string        `yaml:"command"`
	Variables   []Variable    `yaml:"variables,omitempty"`
	Tags        []string      `yaml:"tags,omitempty"`
	Source      SnippetSource `yaml:"-"` // Not persisted to YAML, set during loading
}

// Variable defines a template variable with advanced behavior
type Variable struct {
	Name              string      `yaml:"name"`
	Description       string      `yaml:"description,omitempty"`
	DefaultValue      string      `yaml:"default,omitempty"`
	Required          bool        `yaml:"required,omitempty"`
	Type              string      `yaml:"type,omitempty"`
	Transform         *Transform  `yaml:"transform,omitempty"`
	TransformTemplate string      `yaml:"transform_template,omitempty"`
	Validation        *Validation `yaml:"validation,omitempty"`
	Computed          bool        `yaml:"computed,omitempty"`
}

// Transform defines conditional transformations
type Transform struct {
	EmptyValue   string `yaml:"empty_value,omitempty"`
	ValuePattern string `yaml:"value_pattern,omitempty"`
	TrueValue    string `yaml:"true_value,omitempty"`
	FalseValue   string `yaml:"false_value,omitempty"`
	Compose      string `yaml:"compose,omitempty"`

	composeTpl      *template.Template
	composeTplErr   error
	valuePatternTpl *template.Template
	valuePatternErr error
}

// composeTemplate returns the parsed Compose template, caching the result.
// Returns (nil, nil) when Compose is empty.
func (t *Transform) composeTemplate() (*template.Template, error) {
	if t.Compose == "" {
		return nil, nil
	}
	if t.composeTpl == nil && t.composeTplErr == nil {
		t.composeTpl, t.composeTplErr = template.New("compose").Parse(t.Compose)
	}
	return t.composeTpl, t.composeTplErr
}

// valuePatternTemplate returns the parsed ValuePattern template, caching the result.
// Returns (nil, nil) when ValuePattern is empty.
func (t *Transform) valuePatternTemplate() (*template.Template, error) {
	if t.ValuePattern == "" {
		return nil, nil
	}
	if t.valuePatternTpl == nil && t.valuePatternErr == nil {
		t.valuePatternTpl, t.valuePatternErr = template.New("transform").Parse(t.ValuePattern)
	}
	return t.valuePatternTpl, t.valuePatternErr
}

// Validation defines variable validation rules
type Validation struct {
	Pattern string   `yaml:"pattern,omitempty"`
	Enum    []string `yaml:"enum,omitempty"`
	Range   []int    `yaml:"range,omitempty"`

	patternRE  *regexp.Regexp
	patternErr error
}

// compiledPattern returns the compiled Pattern regex, caching the result.
func (v *Validation) compiledPattern() (*regexp.Regexp, error) {
	if v.patternRE == nil && v.patternErr == nil {
		v.patternRE, v.patternErr = regexp.Compile(v.Pattern)
	}
	return v.patternRE, v.patternErr
}

// TransformTemplate defines a reusable transformation template
type TransformTemplate struct {
	Description string     `yaml:"description"`
	Transform   *Transform `yaml:"transform"`
}

// VariableType defines reusable variable configurations
type VariableType struct {
	Description string      `yaml:"description"`
	Validation  *Validation `yaml:"validation,omitempty"`
	Default     string      `yaml:"default,omitempty"`
	Transform   *Transform  `yaml:"transform,omitempty"`
}

// Config represents the main configuration file
type Config struct {
	TransformTemplates map[string]TransformTemplate `yaml:"transform_templates"`
	VariableTypes      map[string]VariableType      `yaml:"variable_types"`
	Snippets           map[string]Snippet           `yaml:"snippets"`
	Settings           Settings                     `yaml:"settings"`
}

// Settings contains global configuration
type Settings struct {
	AdditionalConfigs []string       `yaml:"additional_configs,omitempty"`
	Selector          SelectorConfig `yaml:"selector"`
}

type SelectorConfig struct {
	Command string `yaml:"command"`
	Options string `yaml:"options"`
}

// ProcessTemplate processes a snippet with variable substitution.
func (s *Snippet) ProcessTemplate(values map[string]string, config *Config) (string, error) {
	processed := make(map[string]string, len(s.Variables))
	for _, variable := range s.Variables {
		result, err := s.ProcessVariable(variable, values[variable.Name], values, config)
		if err != nil {
			return "", fmt.Errorf("processing variable %s: %w", variable.Name, err)
		}
		processed[variable.Name] = result
	}

	return placeholderPattern.ReplaceAllStringFunc(s.Command, func(match string) string {
		name := match[1 : len(match)-1]
		if val, ok := processed[name]; ok {
			return val
		}
		return match
	}), nil
}

// ResolveTransform returns the Transform that applies to this variable, either
// from a named transform_template or the inline definition. Returns nil when
// the variable has no transform. Errors when a named template is missing.
func (v *Variable) ResolveTransform(config *Config) (*Transform, error) {
	if v.TransformTemplate != "" {
		if config == nil {
			return nil, fmt.Errorf("transform template %q requires config", v.TransformTemplate)
		}
		if tmpl, ok := config.TransformTemplates[v.TransformTemplate]; ok {
			return tmpl.Transform, nil
		}
		return nil, fmt.Errorf("transform template '%s' not found", v.TransformTemplate)
	}
	return v.Transform, nil
}

// ProcessVariable applies the variable's transform (if any) to value, using
// allValues as the binding for compose templates.
func (s *Snippet) ProcessVariable(variable Variable, value string, allValues map[string]string, config *Config) (string, error) {
	transform, err := variable.ResolveTransform(config)
	if err != nil {
		return "", err
	}

	if variable.Computed && transform != nil && transform.Compose != "" {
		tmpl, err := transform.composeTemplate()
		if err != nil {
			return "", err
		}
		var buf strings.Builder
		if err := tmpl.Execute(&buf, allValues); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	if transform != nil {
		if variable.Type == VarTypeBoolean {
			if parseBool(value) {
				return transform.TrueValue, nil
			}
			return transform.FalseValue, nil
		}

		if value == "" && transform.EmptyValue != "" {
			return transform.EmptyValue, nil
		}
		if value != "" && transform.ValuePattern != "" {
			tmpl, err := transform.valuePatternTemplate()
			if err != nil {
				return "", err
			}
			var buf strings.Builder
			if err := tmpl.Execute(&buf, map[string]string{"Value": value}); err != nil {
				return "", err
			}
			return buf.String(), nil
		}
	}

	if value == "" {
		return variable.DefaultValue, nil
	}
	return value, nil
}

// Validate checks if variable values meet validation criteria
func (v *Variable) Validate(value string) error {
	if v.Required && value == "" {
		return fmt.Errorf("variable %s is required", v.Name)
	}

	if v.Validation == nil {
		return nil
	}

	// Enum validation
	if len(v.Validation.Enum) > 0 {
		if slices.Contains(v.Validation.Enum, value) {
			return nil
		}
		return fmt.Errorf("variable %s must be one of: %s", v.Name, strings.Join(v.Validation.Enum, ", "))
	}

	// Range validation (for numeric types like ports)
	if len(v.Validation.Range) == 2 && value != "" {
		num, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("variable %s must be a valid number", v.Name)
		}

		lo, hi := v.Validation.Range[0], v.Validation.Range[1]
		if num < lo || num > hi {
			return fmt.Errorf("variable %s must be between %d and %d", v.Name, lo, hi)
		}
	}

	// Pattern validation (regex)
	if v.Validation.Pattern != "" && value != "" {
		re, err := v.Validation.compiledPattern()
		if err != nil {
			return fmt.Errorf("variable %s has invalid pattern: %w", v.Name, err)
		}
		if !re.MatchString(value) {
			return fmt.Errorf("variable %s does not match required format", v.Name)
		}
	}

	return nil
}

// ValidateWithConfig checks validation criteria using config context (for type-based validation)
func (v *Variable) ValidateWithConfig(value string, config *Config) error {
	// First run standard validation
	if err := v.Validate(value); err != nil {
		return err
	}

	// Skip empty values for type validation (unless required, which is handled above)
	if value == "" {
		return nil
	}

	// Special handling for regex type - validate that the value is a valid regex pattern
	if v.Type == VarTypeRegex {
		if _, err := regexp.Compile(value); err != nil {
			return fmt.Errorf("variable %s must be a valid regular expression: %w", v.Name, err)
		}
		return nil
	}

	// Type-based validation using variable_types from config
	if v.Type != "" && config != nil {
		if varType, exists := config.VariableTypes[v.Type]; exists {
			if varType.Validation != nil {
				// Create a temporary variable with the type's validation rules
				tempVar := Variable{
					Name:       v.Name,
					Type:       v.Type,
					Validation: varType.Validation,
				}
				return tempVar.Validate(value)
			}
		}
	}

	return nil
}
