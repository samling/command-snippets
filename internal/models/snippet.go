package models

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// SnippetSource represents where a snippet was loaded from
type SnippetSource string

const (
	SourceGlobal SnippetSource = "global"
	SourceLocal  SnippetSource = "local"
)

// Snippet represents a command template
type Snippet struct {
	ID          string        `yaml:"id"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Command     string        `yaml:"command"`
	Variables   []Variable    `yaml:"variables,omitempty"`
	Tags        []string      `yaml:"tags,omitempty"`
	CreatedAt   time.Time     `yaml:"created_at"`
	UpdatedAt   time.Time     `yaml:"updated_at"`
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
	TransformTemplate string      `yaml:"transformTemplate,omitempty"`
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
}

// Validation defines variable validation rules
type Validation struct {
	Pattern string   `yaml:"pattern,omitempty"`
	Enum    []string `yaml:"enum,omitempty"`
	Range   []int    `yaml:"range,omitempty"`
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
	AdditionalConfigs []string          `yaml:"additional_configs,omitempty"`
	Interactive       InteractiveConfig `yaml:"interactive"`
	Selector          SelectorConfig    `yaml:"selector"`
}

type InteractiveConfig struct {
	ConfirmBeforeExecute bool `yaml:"confirm_before_execute"`
	ShowFinalCommand     bool `yaml:"show_final_command"`
}

type SelectorConfig struct {
	Command string `yaml:"command"`
	Options string `yaml:"options"`
}

// ProcessTemplate processes a snippet with variable substitution
func (s *Snippet) ProcessTemplate(values map[string]string, config *Config) (string, error) {
	command := s.Command

	// Process each variable defined in the snippet
	for _, variable := range s.Variables {
		placeholder := fmt.Sprintf("<%s>", variable.Name)
		value := values[variable.Name]

		processedValue, err := s.processVariable(variable, value, values, config)
		if err != nil {
			return "", fmt.Errorf("processing variable %s: %w", variable.Name, err)
		}

		command = strings.ReplaceAll(command, placeholder, processedValue)
	}

	return command, nil
}

// processVariable handles individual variable transformation
func (s *Snippet) processVariable(variable Variable, value string, allValues map[string]string, config *Config) (string, error) {
	// Determine which transform to use
	var transform *Transform

	// Use transformTemplate if specified
	if variable.TransformTemplate != "" {
		if tmplDef, exists := config.TransformTemplates[variable.TransformTemplate]; exists {
			transform = tmplDef.Transform
		} else {
			return "", fmt.Errorf("transform template '%s' not found", variable.TransformTemplate)
		}
	} else if variable.Transform != nil {
		// Use inline transform
		transform = variable.Transform
	}

	// Handle computed variables first
	if variable.Computed && transform != nil && transform.Compose != "" {
		tmpl, err := template.New("compose").Parse(transform.Compose)
		if err != nil {
			return "", err
		}

		var buf strings.Builder
		if err := tmpl.Execute(&buf, allValues); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	// Handle transformations
	if transform != nil {
		// Boolean transformations
		if variable.Type == "boolean" {
			if value == "true" || value == "yes" || value == "1" {
				return transform.TrueValue, nil
			}
			return transform.FalseValue, nil
		}

		// Regular transformations
		if value == "" && transform.EmptyValue != "" {
			return transform.EmptyValue, nil
		} else if value != "" && transform.ValuePattern != "" {
			tmpl, err := template.New("transform").Parse(transform.ValuePattern)
			if err != nil {
				return "", err
			}

			var buf strings.Builder
			data := map[string]string{"Value": value}
			if err := tmpl.Execute(&buf, data); err != nil {
				return "", err
			}
			return buf.String(), nil
		}
	}

	// Use default value if empty
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
		for _, allowed := range v.Validation.Enum {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("variable %s must be one of: %s", v.Name, strings.Join(v.Validation.Enum, ", "))
	}

	// Range validation (for numeric types like ports)
	if len(v.Validation.Range) == 2 && value != "" {
		var num int
		if _, err := fmt.Sscanf(value, "%d", &num); err != nil {
			return fmt.Errorf("variable %s must be a valid number", v.Name)
		}

		min, max := v.Validation.Range[0], v.Validation.Range[1]
		if num < min || num > max {
			return fmt.Errorf("variable %s must be between %d and %d", v.Name, min, max)
		}
	}

	// Pattern validation (regex)
	if v.Validation.Pattern != "" && value != "" {
		matched, err := regexp.MatchString(v.Validation.Pattern, value)
		if err != nil {
			return fmt.Errorf("variable %s has invalid pattern: %v", v.Name, err)
		}
		if !matched {
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
	if v.Type == "regex" {
		_, err := regexp.Compile(value)
		if err != nil {
			return fmt.Errorf("variable %s must be a valid regular expression: %v", v.Name, err)
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
