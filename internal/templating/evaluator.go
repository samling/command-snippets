package templating

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
)

var interpolationPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

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
	var firstErr error
	result := interpolationPattern.ReplaceAllStringFunc(input, func(match string) string {
		if firstErr != nil {
			return match
		}
		expression := strings.TrimSpace(match[2 : len(match)-1])
		value, ok := values[expression]
		if ok {
			return value
		}
		out, err := EvalString(expression, values)
		if err != nil {
			firstErr = fmt.Errorf("interpolation %s: %w", match, err)
			return match
		}
		return out
	})
	if firstErr != nil {
		return "", firstErr
	}
	return result, nil
}

func NormalizeCommandWhitespace(command string) string {
	return strings.Join(strings.Fields(command), " ")
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
