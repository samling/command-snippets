package templating

import (
	"fmt"
	"strings"
)

func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func quoteIfUnsafe(value string) string {
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && !strings.ContainsRune("_@%+=:,./-", r) {
			return quote(value)
		}
	}
	return value
}
func flag(name, value string) string {
	if value == "" {
		return ""
	}
	return name + " " + quoteIfUnsafe(value)
}
func boolFlag(name string, enabled bool) string {
	if enabled {
		return name
	}
	return ""
}
func repeatFlag(name string, values []string) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = flag(name, value)
	}
	return strings.Join(parts, " ")
}
func joinNonEmpty(values []string, separator string) string {
	parts := []string{}
	for _, value := range values {
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, separator)
}
func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func empty(value any) bool {
	switch v := value.(type) {
	case string:
		return v == ""
	case bool:
		return !v
	case []string:
		return len(v) == 0
	case []any:
		return len(v) == 0
	case nil:
		return true
	}
	return false
}

// Expr creates []any for list literals, while repeat inputs use []string.
// Guard this runtime boundary in addition to the AST's string-list checking.
func expressionStrings(value any) ([]string, error) {
	switch list := value.(type) {
	case []string:
		return list, nil
	case []any:
		items := make([]string, len(list))
		for i, item := range list {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("list item %d must be text", i+1)
			}
			items[i] = str
		}
		return items, nil
	}
	return nil, fmt.Errorf("expected a string list")
}
func expressionJoin(value any, separator string) (string, error) {
	items, err := expressionStrings(value)
	if err != nil {
		return "", err
	}
	return joinNonEmpty(items, separator), nil
}
func expressionRepeatFlag(name string, value any) (string, error) {
	items, err := expressionStrings(value)
	if err != nil {
		return "", err
	}
	return repeatFlag(name, items), nil
}
