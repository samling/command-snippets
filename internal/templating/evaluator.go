package templating

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/expr-lang/expr"
)

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
