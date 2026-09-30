package library

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/samling/command-snippets/internal/models"
	"gopkg.in/yaml.v3"
)

// YAML's string decoder coerces scalars such as numbers and booleans. The
// public schema does not: quoted numeric defaults/choice outputs remain text.
func checkSchemaTypes(node *yaml.Node, kind reflect.Type) error {
	for kind.Kind() == reflect.Pointer {
		kind = kind.Elem()
	}
	if kind.Kind() == reflect.Interface {
		return nil
	}
	if node.Kind == yaml.DocumentNode {
		return checkSchemaTypes(node.Content[0], kind)
	}
	invalid := func() error { return fmt.Errorf("line %d: expected %s, got %s", node.Line, kind.String(), node.Tag) }
	switch kind.Kind() {
	case reflect.Struct:
		if kind == reflect.TypeOf(models.Input{}) {
			if value := mapValue(node, "default"); value != nil && value.Tag == "!!null" {
				return fmt.Errorf("line %d: default must have the input's type, not null", value.Line)
			}
		}
		if node.Kind != yaml.MappingNode {
			return invalid()
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < kind.NumField(); i++ {
			field := kind.Field(i)
			name := strings.Split(field.Tag.Get("yaml"), ",")[0]
			fields[name] = field.Type
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if field, found := fields[node.Content[i].Value]; found {
				if err := checkSchemaTypes(node.Content[i+1], field); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("line %d: unknown field %q in %s", node.Content[i].Line, node.Content[i].Value, kind.Name())
			}
		}
	case reflect.Slice:
		if node.Kind != yaml.SequenceNode {
			return invalid()
		}
		for _, child := range node.Content {
			if err := checkSchemaTypes(child, kind.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		if node.Kind != yaml.MappingNode {
			return invalid()
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if err := checkSchemaTypes(node.Content[i], kind.Key()); err != nil {
				return err
			}
			if err := checkSchemaTypes(node.Content[i+1], kind.Elem()); err != nil {
				return err
			}
		}
	case reflect.String:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return invalid()
		}
	case reflect.Bool:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
			return invalid()
		}
	case reflect.Int:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
			return invalid()
		}
	}
	return nil
}
func validateDocumentTypes(node *yaml.Node) error {
	return checkSchemaTypes(node, reflect.TypeOf(models.Config{}))
}
