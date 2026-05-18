package templating

import (
	"fmt"
	"strconv"
	"strings"
)

func flag(name string, value any) string {
	s := strings.TrimSpace(toString(value))
	if s == "" {
		return ""
	}
	return strings.TrimSpace(name + " " + s)
}

func boolFlag(name string, enabled any) string {
	if truthy(enabled) {
		return name
	}
	return ""
}

func quote(value any) string {
	return strconv.Quote(toString(value))
}

func joinNonEmpty(values []string, sep string) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, sep)
}

func defaultValue(value any, fallback any) string {
	if strings.TrimSpace(toString(value)) == "" {
		return toString(fallback)
	}
	return toString(value)
}

func empty(value any) bool {
	return strings.TrimSpace(toString(value)) == ""
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func truthy(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "1", "on":
			return true
		}
		return false
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return value != nil
	}
}
