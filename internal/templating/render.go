package templating

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/samling/command-snippets/internal/models"
)

// PreviewPart carries presentation metadata, never an executable partial command.
type PreviewPart struct {
	Text     string
	Value    bool
	Unfilled bool
	// Name is the placeholder's input or expression name; empty for literal text.
	Name string
	// Inputs are the inputs this part derives from, in snippet order: the
	// placeholder's own input, or an expression's compiler-discovered inputs.
	Inputs []string
	// Missing lists the Inputs whose current value blocks an unfilled part.
	Missing []string
}

type Result struct {
	Preview  []PreviewPart
	Command  string
	Errors   map[string]string
	Values   map[string]any
	Visible  map[string]bool
	Required map[string]bool
}

func (r Result) Valid() bool { return len(r.Errors) == 0 }
func (r Result) Error() error {
	if r.Valid() {
		return nil
	}
	keys := []string{}
	for key := range r.Errors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	messages := []string{}
	for _, key := range keys {
		messages = append(messages, key+": "+r.Errors[key])
	}
	return fmt.Errorf("%s", strings.Join(messages, "; "))
}
func validateInput(in models.Input, value any, required bool) error {
	switch in.InputKind() {
	case "toggle":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be true or false")
		}
	case "repeat":
		if _, ok := value.([]string); !ok {
			return fmt.Errorf("must be a list of strings")
		}
	default:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("must be text")
		}
	}
	if required && empty(value) {
		return fmt.Errorf("is required")
	}
	if in.InputKind() == "choice" {
		label := value.(string)
		found := false
		for _, c := range in.Choices {
			if c.Label == label {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("choose one of the configured labels")
		}
	}
	values := []string{}
	switch v := value.(type) {
	case string:
		if v != "" {
			values = append(values, v)
		}
	case []string:
		values = v
	}
	for _, s := range values {
		if s == "" {
			return fmt.Errorf("repeat items cannot be empty")
		}
		if strings.ContainsRune(s, 0) {
			return fmt.Errorf("NUL cannot be represented in a shell argument")
		}
		if v := in.Validate; v != nil {
			if v.Pattern != "" {
				re, err := regexp.Compile(v.Pattern)
				if err != nil {
					return err
				}
				if !re.MatchString(s) {
					return fmt.Errorf("must match %s", v.Pattern)
				}
			}
			if v.Range != nil {
				n, err := strconv.Atoi(s)
				if err != nil || n < v.Range[0] || n > v.Range[1] {
					return fmt.Errorf("must be an integer in [%d, %d]", v.Range[0], v.Range[1])
				}
			}
			if v.Regex {
				if _, err := regexp.Compile(s); err != nil {
					return fmt.Errorf("invalid regex: %w", err)
				}
			}
		}
	}
	return nil
}
func evaluateCondition(condition compiledCondition, values map[string]any) (bool, error) {
	if condition.definition == nil {
		return true, nil
	}
	if condition.program != nil {
		out, err := expr.Run(condition.program, expressionEnv(values))
		if err != nil {
			return false, err
		}
		b, ok := out.(bool)
		if !ok {
			return false, fmt.Errorf("condition did not return boolean")
		}
		return b, nil
	}
	c := condition.definition
	if c.Equals != nil {
		return reflect.DeepEqual(values[c.Input], c.Equals), nil
	}
	return !reflect.DeepEqual(values[c.Input], c.NotEquals), nil
}
func inputOutput(in models.Input, value any) string {
	if str, ok := value.(string); ok {
		if output, found := in.Special[str]; found {
			return output
		}
	}
	switch in.InputKind() {
	case "toggle":
		return boolFlag(in.Flag, value.(bool))
	case "flag":
		return flag(in.Flag, value.(string))
	case "repeat":
		return repeatFlag(in.Flag, value.([]string))
	case "choice":
		for _, choice := range in.Choices {
			if choice.Label == value.(string) {
				return choice.Value
			}
		}
		return ""
	default:
		return quoteIfUnsafe(value.(string))
	}
}
func (t *Template) Preview(presets map[string]any) Result {
	result := Result{Errors: map[string]string{}, Values: map[string]any{}, Visible: map[string]bool{}, Required: map[string]bool{}}
	known := map[string]bool{}
	for _, in := range t.Snippet.Inputs {
		known[in.Name] = true
		value := in.DefaultValue()
		if preset, found := presets[in.Name]; found {
			value = preset
		}
		result.Values[in.Name] = value
	}
	for name := range presets {
		if !known[name] {
			result.Errors[name] = "unknown input"
		}
	}
	// Type errors are caught before expressions evaluate; hidden inputs are cleared below.
	for _, in := range t.Snippet.Inputs {
		value := result.Values[in.Name]
		switch in.InputKind() {
		case "toggle":
			if _, ok := value.(bool); !ok {
				result.Errors[in.Name] = "must be boolean"
				result.Values[in.Name] = in.Zero()
			}
		case "repeat":
			if _, ok := value.([]string); !ok {
				result.Errors[in.Name] = "must be a string list"
				result.Values[in.Name] = in.Zero()
			}
		default:
			if _, ok := value.(string); !ok {
				result.Errors[in.Name] = "must be text"
				result.Values[in.Name] = in.Zero()
			}
		}
	}
	fragments := map[string]string{}
	for _, idx := range t.order {
		in := t.Snippet.Inputs[idx]
		visible, err := evaluateCondition(t.visible[in.Name], result.Values)
		if err != nil {
			result.Errors[in.Name] = "visibility: " + err.Error()
			visible = false
		}
		result.Visible[in.Name] = visible
		if !visible {
			result.Values[in.Name] = in.Zero()
			fragments[in.Name] = ""
			continue
		}
		required := in.Required
		if condition, exists := t.required[in.Name]; exists {
			matched, err := evaluateCondition(condition, result.Values)
			if err != nil {
				result.Errors[in.Name] = "requiredness: " + err.Error()
			} else {
				required = required || matched
			}
		}
		result.Required[in.Name] = required
		if _, bad := result.Errors[in.Name]; !bad {
			if err := validateInput(in, result.Values[in.Name], required); err != nil {
				result.Errors[in.Name] = err.Error()
			} else {
				fragments[in.Name] = inputOutput(in, result.Values[in.Name])
			}
		}
	}
	// An unrelated invalid input must not hide valid expression fragments.
	// Expressions with invalid dependencies remain unresolved in the preview.
	for name, program := range t.expressions {
		validInputs := true
		for _, input := range t.expressionInputs[name] {
			if _, invalid := result.Errors[input]; invalid {
				validInputs = false
				break
			}
		}
		if !validInputs {
			continue
		}
		output, err := expr.Run(program, expressionEnv(result.Values))
		if err != nil {
			result.Errors[name] = err.Error()
			continue
		}
		str, ok := output.(string)
		if !ok {
			result.Errors[name] = "expression must return string"
			continue
		}
		fragments[name] = str
	}
	var command strings.Builder
	for _, segment := range t.segments {
		part := PreviewPart{Text: segment.literal}
		if segment.name != "" {
			part.Value = true
			part.Name = segment.name
			part.Inputs = t.partInputs(segment.name)
			var found bool
			part.Text, found = fragments[segment.name]
			if !found {
				part.Text = "{{" + segment.name + "}}"
				part.Unfilled = true
				for _, input := range part.Inputs {
					if _, invalid := result.Errors[input]; invalid {
						part.Missing = append(part.Missing, input)
					}
				}
			}
		}
		command.WriteString(part.Text)
		if command.Len() > 1<<20 {
			result.Errors["command"] = "rendered command exceeds 1 MiB"
			result.Preview = nil
			return result
		}
		result.Preview = append(result.Preview, part)
	}
	if !result.Valid() {
		return result
	}
	result.Command = command.String()
	if strings.ContainsRune(result.Command, 0) {
		result.Errors["command"] = "NUL cannot appear in a shell command"
		result.Command = ""
	}
	return result
}

// Uses lists the inputs a computed value (expression) reads, in snippet order.
func (t *Template) Uses(name string) []string { return t.partInputs(name) }

// partInputs names the inputs a placeholder derives from, in snippet order.
func (t *Template) partInputs(name string) []string {
	deps, isExpression := t.expressionInputs[name]
	if !isExpression {
		return []string{name}
	}
	uses := map[string]bool{}
	for _, dep := range deps {
		uses[dep] = true
	}
	ordered := []string{}
	for _, in := range t.Snippet.Inputs {
		if uses[in.Name] {
			ordered = append(ordered, in.Name)
		}
	}
	return ordered
}
func (t *Template) Render(values map[string]any) (string, error) {
	result := t.Preview(values)
	if err := result.Error(); err != nil {
		return "", err
	}
	return result.Command, nil
}

// Presets preserves item boundaries, explicit empties, and values containing '='.
func Presets(snippet models.Snippet, arguments []string) (map[string]any, error) {
	result := map[string]any{}
	inputs := map[string]models.Input{}
	for _, in := range snippet.Inputs {
		inputs[in.Name] = in
	}
	for _, argument := range arguments {
		name, value, ok := strings.Cut(argument, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("--set needs name=value")
		}
		in, exists := inputs[name]
		if !exists {
			return nil, fmt.Errorf("unknown input %q", name)
		}
		if in.InputKind() == "repeat" {
			items, _ := result[name].([]string)
			result[name] = append(items, value)
			continue
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("input %s supplied more than once", name)
		}
		if in.InputKind() == "toggle" {
			switch strings.ToLower(value) {
			case "true":
				result[name] = true
			case "false":
				result[name] = false
			default:
				return nil, fmt.Errorf("input %s must be true or false", name)
			}
		} else {
			result[name] = value
		}
	}
	return result, nil
}
