package templating

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/expr-lang/expr"
)

type ComputedValue struct {
	// Value is evaluated as an expr expression; quote string literals explicitly.
	Value string `yaml:"value,omitempty"`
	// Case values are interpolation text rendered with Interpolate.
	Cases []ComputedCase `yaml:"cases,omitempty"`
}

type ComputedCase struct {
	When    string `yaml:"when,omitempty"`
	Value   string `yaml:"value,omitempty"`
	Default bool   `yaml:"default,omitempty"`
}

func EvalBool(expression string, values map[string]string) (bool, error) {
	if strings.TrimSpace(expression) == "" {
		return true, nil
	}
	out, err := eval(expression, values)
	if err != nil {
		return false, err
	}
	b, ok := out.(bool)
	if !ok {
		return false, fmt.Errorf("expression must return boolean, got %T", out)
	}
	return b, nil
}

func EvalString(expression string, values map[string]string) (string, error) {
	out, err := eval(expression, values)
	if err != nil {
		return "", err
	}
	return toString(out), nil
}

func Interpolate(input string, values map[string]string) (string, error) {
	var result strings.Builder
	for i := 0; i < len(input); {
		if i+1 >= len(input) || input[i] != '$' || input[i+1] != '{' {
			result.WriteByte(input[i])
			i++
			continue
		}

		expression, end, ok := scanInterpolationExpression(input, i+2)
		if !ok {
			result.WriteString(input[i:])
			break
		}

		expression = strings.TrimSpace(expression)
		if value, ok := values[expression]; ok {
			result.WriteString(value)
			i = end + 1
			continue
		}

		out, err := EvalString(expression, values)
		if err != nil {
			return "", fmt.Errorf("interpolation %s: %w", input[i:end+1], err)
		}
		result.WriteString(out)
		i = end + 1
	}
	return result.String(), nil
}

func NormalizeCommandWhitespace(command string) string {
	var result strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	wroteSpace := false

	for _, r := range command {
		if escaped {
			result.WriteRune(r)
			escaped = false
			wroteSpace = false
			continue
		}

		if r == '\\' {
			result.WriteRune(r)
			escaped = true
			wroteSpace = false
			continue
		}

		switch r {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
			result.WriteRune(r)
			wroteSpace = false
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
			result.WriteRune(r)
			wroteSpace = false
		default:
			if unicode.IsSpace(r) && !inSingleQuote && !inDoubleQuote {
				if result.Len() > 0 && !wroteSpace {
					result.WriteByte(' ')
					wroteSpace = true
				}
				continue
			}
			result.WriteRune(r)
			wroteSpace = false
		}
	}
	return strings.TrimSpace(result.String())
}

func ResolveComputed(computed map[string]ComputedValue, values map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(computed))
	ctx := make(map[string]string, len(values)+len(computed))
	for k, v := range values {
		ctx[k] = v
	}

	names := make([]string, 0, len(computed))
	for name := range computed {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		item := computed[name]
		value, err := resolveComputedValue(name, item, ctx)
		if err != nil {
			return nil, err
		}
		resolved[name] = value
		ctx[name] = value
	}

	return resolved, nil
}

func resolveComputedValue(name string, item ComputedValue, values map[string]string) (string, error) {
	if len(item.Cases) == 0 {
		value, err := EvalString(item.Value, values)
		if err != nil {
			return "", fmt.Errorf("computed %s value: %w", name, err)
		}
		return value, nil
	}

	defaultIndex := -1
	for i, computedCase := range item.Cases {
		if computedCase.Default {
			if defaultIndex == -1 {
				defaultIndex = i
			}
			continue
		}

		matched, err := EvalBool(computedCase.When, values)
		if err != nil {
			return "", fmt.Errorf("computed %s case %d when: %w", name, i, err)
		}
		if matched {
			value, err := Interpolate(computedCase.Value, values)
			if err != nil {
				return "", fmt.Errorf("computed %s case %d value: %w", name, i, err)
			}
			return value, nil
		}
	}

	if defaultIndex != -1 {
		value, err := Interpolate(item.Cases[defaultIndex].Value, values)
		if err != nil {
			return "", fmt.Errorf("computed %s case %d value: %w", name, defaultIndex, err)
		}
		return value, nil
	}

	return "", nil
}

func scanInterpolationExpression(input string, start int) (string, int, bool) {
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	depth := 0

	for i := start; i < len(input); i++ {
		c := input[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if inSingleQuote || inDoubleQuote {
			continue
		}

		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case '}':
			if depth == 0 {
				return input[start:i], i, true
			}
			depth--
		}
	}
	return "", 0, false
}

func eval(expression string, values map[string]string) (any, error) {
	env := make(map[string]any, len(values)+6)
	for k, v := range values {
		env[k] = v
	}
	env["flag"] = flag
	env["boolFlag"] = boolFlag
	env["quote"] = quote
	env["join"] = joinNonEmpty
	env["default"] = defaultValue
	env["empty"] = empty

	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, err
	}
	return expr.Run(program, env)
}
