package templating

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/expr-lang/expr/vm"
	"github.com/samling/command-snippets/internal/models"
)

type segment struct{ literal, name string }
type compiledCondition struct {
	definition *models.Condition
	program    *vm.Program
}
type Template struct {
	Snippet     models.Snippet
	segments    []segment
	expressions map[string]*vm.Program
	visible     map[string]compiledCondition
	required    map[string]compiledCondition
	order       []int
}

// Compile is the only entry point for template/schema validation on every surface.
func Compile(snippet models.Snippet) (*Template, error) {
	if err := snippet.Validate(); err != nil {
		return nil, err
	}
	template := &Template{Snippet: snippet, expressions: map[string]*vm.Program{}, visible: map[string]compiledCondition{}, required: map[string]compiledCondition{}}
	names := map[string]models.Input{}
	allNames := map[string]bool{}
	for _, in := range snippet.Inputs {
		names[in.Name] = in
		allNames[in.Name] = true
		if err := validateInput(in, in.DefaultValue(), false); err != nil {
			return nil, fmt.Errorf("input %s default: %w", in.Name, err)
		}
	}
	for name, source := range snippet.Expressions {
		allNames[name] = true
		program, _, err := compileExpression(source, names, false)
		if err != nil {
			return nil, fmt.Errorf("expression %s: %w", name, err)
		}
		template.expressions[name] = program
	}
	segments, err := parseCommand(snippet.Command, allNames)
	if err != nil {
		return nil, err
	}
	template.segments = segments
	dependencies := map[string][]string{}
	for _, in := range snippet.Inputs {
		for _, pair := range []struct {
			condition *models.Condition
			target    map[string]compiledCondition
		}{{in.VisibleWhen, template.visible}, {in.RequiredWhen, template.required}} {
			c := pair.condition
			if c == nil {
				continue
			}
			compiled := compiledCondition{definition: c}
			if c.Expr != "" {
				program, deps, err := compileExpression(c.Expr, names, true)
				if err != nil {
					return nil, fmt.Errorf("input %s condition: %w", in.Name, err)
				}
				compiled.program = program
				dependencies[in.Name] = append(dependencies[in.Name], deps...)
			} else {
				referenced, ok := names[c.Input]
				if !ok {
					return nil, fmt.Errorf("input %s: unknown condition input %s", in.Name, c.Input)
				}
				comparison := c.Equals
				if comparison == nil {
					comparison = c.NotEquals
				}
				if items, ok := comparison.([]any); ok && referenced.InputKind() == "repeat" {
					strings := make([]string, len(items))
					for i, item := range items {
						value, ok := item.(string)
						if !ok {
							return nil, fmt.Errorf("input %s: comparison items must be strings", in.Name)
						}
						strings[i] = value
					}
					comparison = strings
				}
				definition := *c
				if c.Equals != nil {
					definition.Equals = comparison
				} else {
					definition.NotEquals = comparison
				}
				compiled.definition = &definition
				if reflect.TypeOf(comparison) != reflect.TypeOf(referenced.Zero()) {
					return nil, fmt.Errorf("input %s: condition value must have the same type as %s", in.Name, c.Input)
				}
				dependencies[in.Name] = append(dependencies[in.Name], c.Input)
			}
			pair.target[in.Name] = compiled
		}
	}
	state := map[string]int{}
	indices := map[string]int{}
	for i, in := range snippet.Inputs {
		indices[in.Name] = i
	}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("condition dependency cycle at input %s", name)
		}
		if state[name] == 2 {
			return nil
		}
		state[name] = 1
		sort.Strings(dependencies[name])
		for _, dep := range dependencies[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		template.order = append(template.order, indices[name])
		return nil
	}
	for _, in := range snippet.Inputs {
		if err := visit(in.Name); err != nil {
			return nil, err
		}
	}
	return template, nil
}
func parseCommand(command string, names map[string]bool) ([]segment, error) {
	result := []segment{}
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			result = append(result, segment{literal: literal.String()})
			literal.Reset()
		}
	}
	for pos := 0; pos < len(command); {
		if strings.HasPrefix(command[pos:], "{{{{") {
			literal.WriteString("{{")
			pos += 4
			continue
		}
		if strings.HasPrefix(command[pos:], "{{") {
			end := strings.Index(command[pos+2:], "}}")
			if end < 0 {
				return nil, fmt.Errorf("unclosed placeholder at byte %d", pos)
			}
			end += pos + 2
			name := strings.TrimSpace(command[pos+2 : end])
			if !models.ValidIdentifier(name) || !names[name] {
				return nil, fmt.Errorf("unknown or invalid placeholder %q at byte %d; define an input or expression", name, pos)
			}
			flush()
			result = append(result, segment{name: name})
			pos = end + 2
			continue
		}
		literal.WriteByte(command[pos])
		pos++
	}
	flush()
	return result, nil
}

// Placeholders discovers names for guided authoring without compiling an incomplete draft.
func Placeholders(command string) []string {
	names := []string{}
	seen := map[string]bool{}
	for pos := 0; pos < len(command); {
		if strings.HasPrefix(command[pos:], "{{{{") {
			pos += 4
			continue
		}
		if !strings.HasPrefix(command[pos:], "{{") {
			pos++
			continue
		}
		end := strings.Index(command[pos+2:], "}}")
		if end < 0 {
			break
		}
		end += pos + 2
		name := strings.TrimSpace(command[pos+2 : end])
		if models.ValidIdentifier(name) && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
		pos = end + 2
	}
	return names
}
