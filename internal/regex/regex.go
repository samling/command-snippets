package regex

import (
	"fmt"
	"strings"
)

func ExplainRegexPattern(pattern string) string {
	if pattern == "" {
		return ""
	}

	var result strings.Builder
	inCaptureGroup := 0

	for i := 0; i < len(pattern); i++ {
		char := pattern[i]
		var styledChar, explanation string

		// Handle escape sequences
		if char == '\\' && i < len(pattern)-1 {
			nextChar := pattern[i+1]
			styledChar = fmt.Sprintf("\\%c", nextChar)

			// Expanded escape sequence explanations
			switch nextChar {
			case 'w':
				explanation = "Match any word character [a-zA-Z0-9_]"
			case 'W':
				explanation = "Match any non-word character"
			case 'd':
				explanation = "Match any digit [0-9]"
			case 'D':
				explanation = "Match any non-digit"
			case 's':
				explanation = "Match any whitespace character (space, tab, newline)"
			case 'S':
				explanation = "Match any non-whitespace character"
			case 'b':
				explanation = "Match a word boundary"
			case 'B':
				explanation = "Match a non-word boundary"
			case 'A':
				explanation = "Match start of string"
			case 'z':
				explanation = "Match end of string"
			case 't':
				explanation = "Match tab character"
			case 'n':
				explanation = "Match newline character"
			case 'r':
				explanation = "Match carriage return"
			default:
				explanation = fmt.Sprintf("Escape sequence for '%c'", nextChar)
			}
			i++ // Skip the next character
		} else if char == '{' {
			// Look ahead to parse the complete quantifier
			end := i + 1
			isValidQuantifier := false
			// First, validate that this is a proper quantifier
			for j := end; j < len(pattern); j++ {
				if pattern[j] == '}' {
					// Check if content between braces is valid (digits and maybe a comma)
					content := pattern[end:j]
					if isValidQuantifierContent(content) {
						end = j
						isValidQuantifier = true
						break
					}
				}
			}

			if isValidQuantifier {
				quantifier := pattern[i : end+1]
				styledChar = quantifier
				explanation = getQuantifierExplanation(pattern[i : end+1])
				i = end // Skip to end of quantifier
			} else {
				styledChar = fmt.Sprintf("%c", char)
				explanation = "Match this character literally"
			}
		} else if isMetaChar(char) {
			styledChar = fmt.Sprintf("%c", char)
			explanation = getCharacterExplanation(pattern, i, rune(char))
		} else if char == '(' || char == ')' {
			styledChar = fmt.Sprintf("%c", char)
			explanation = getCharacterExplanation(pattern, i, rune(char))
			// Update capture group nesting level
			if char == '(' && (i >= len(pattern)-2 || pattern[i+1] != '?' || pattern[i+2] != ':') {
				inCaptureGroup++
			} else if char == ')' {
				inCaptureGroup = max(0, inCaptureGroup-1)
			}
		} else if isQuantifier(char) {
			styledChar = fmt.Sprintf("%c", char)
			explanation = getCharacterExplanation(pattern, i, rune(char))
		} else {
			styledChar = fmt.Sprintf("%c", char)
			explanation = "Match this character literally"
		}

		line := fmt.Sprintf("%s: %s\n", styledChar, explanation)
		// Add indentation if we're inside a capture group (but not for the group markers themselves)
		if inCaptureGroup > 0 && char != '(' && char != ')' {
			line = "  " + line
		}
		result.WriteString(line)
	}
	return result.String()
}

// Helper function to get maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isMetaChar(c byte) bool {
	return c == '^' || c == '$' || c == '.' || c == '[' || c == ']' || c == '|'
}

func isQuantifier(c byte) bool {
	return c == '*' || c == '+' || c == '?' || c == '{' || c == '}'
}

func getCharacterExplanation(pattern string, pos int, char rune) string {
	switch char {
	case '^':
		return "Match start of line"
	case '$':
		return "Match end of line"
	case '.':
		return "Match any character except newline"
	case '*':
		if pos > 0 {
			return "Match previous character zero or more times"
		}
		return "Match previous character zero or more times"
	case '+':
		if pos > 0 {
			return "Match previous character one or more times"
		}
		return "Match previous character one or more times"
	case '?':
		if pos > 0 {
			return "Match previous character zero or one time"
		}
		return "Match previous character zero or one time"
	case '[':
		return "Start character class"
	case ']':
		return "End character class"
	case '(':
		if pos < len(pattern)-2 && pattern[pos+1] == '?' && pattern[pos+2] == ':' {
			return "Start non-capturing group"
		}
		return "Start capturing group"
	case ')':
		return "End group"
	case '{':
		// Look ahead to parse the quantifier
		if pos < len(pattern)-1 {
			end := pos + 1
			for end < len(pattern) && pattern[end] != '}' {
				end++
			}
			if end < len(pattern) {
				quantifier := pattern[pos+1 : end]
				parts := strings.Split(quantifier, ",")

				if len(parts) == 2 {
					if parts[1] == "" {
						return fmt.Sprintf("Match previous character at least %s times", parts[0])
					}
					return fmt.Sprintf("Match previous character between %s and %s times", parts[0], parts[1])
				} else if len(parts) == 1 && parts[0] != "" {
					return fmt.Sprintf("Match previous character exactly %s times", parts[0])
				}
			}
		}
		return "Invalid quantifier"
	case '}':
		// Look backwards to see if this closes a valid quantifier
		start := pos - 1
		for start >= 0 && pattern[start] != '{' {
			start--
		}
		if start >= 0 && start < pos {
			// Check if the content between { and } is a valid quantifier
			content := pattern[start+1 : pos]
			if isValidQuantifierContent(content) {
				return "Closes quantifier range"
			}
		}
		return "Match this character literally"
	case '|':
		return "Alternation (OR)"
	default:
		return "Match this character literally"
	}
}

// Add new helper function
func getQuantifierExplanation(quantifier string) string {
	// Strip the braces
	content := quantifier[1 : len(quantifier)-1]
	parts := strings.Split(content, ",")

	if len(parts) == 2 {
		if parts[1] == "" {
			return fmt.Sprintf("Match %s or more of the preceding token", parts[0])
		}
		return fmt.Sprintf("Match between %s and %s of the preceding token", parts[0], parts[1])
	} else if len(parts) == 1 && parts[0] != "" {
		return fmt.Sprintf("Match exactly %s of the preceding token", parts[0])
	}
	return "Invalid quantifier"
}

// Add new helper function to validate quantifier content
func isValidQuantifierContent(content string) bool {
	if content == "" {
		return false
	}

	parts := strings.Split(content, ",")
	if len(parts) > 2 {
		return false
	}

	// Check first number
	if !isNumeric(parts[0]) {
		return false
	}

	// If there's a second part (for ranges like {2,4})
	if len(parts) == 2 {
		// Allow empty second part for {n,} syntax
		if parts[1] != "" && !isNumeric(parts[1]) {
			return false
		}
	}

	return true
}

// Helper function to check if a string is numeric
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
