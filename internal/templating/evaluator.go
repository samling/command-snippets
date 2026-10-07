package templating

import (
	"fmt"
	"reflect"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/conf"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
	"github.com/samling/command-snippets/internal/models"
)

type expressionCheck struct {
	names        map[string]models.Input
	dependencies map[string]bool
	err          error
	nodes        int
}

func (v *expressionCheck) Visit(node *ast.Node) {
	v.nodes++
	if v.nodes > 1000 {
		v.err = fmt.Errorf("expression exceeds 1000 nodes")
		return
	}
	switch n := (*node).(type) {
	case *ast.MemberNode:
		root, ok := n.Node.(*ast.IdentifierNode)
		prop, pok := n.Property.(*ast.StringNode)
		if !ok || !pok || root.Value != "inputs" || n.Method || n.Optional {
			v.err = fmt.Errorf("only inputs.name access is supported")
			return
		}
		if _, exists := v.names[prop.Value]; !exists {
			v.err = fmt.Errorf("unknown input %q", prop.Value)
			return
		}
		v.dependencies[prop.Value] = true
	case *ast.CallNode:
		name, ok := n.Callee.(*ast.IdentifierNode)
		if !ok || !helperName(name.Value) {
			v.err = fmt.Errorf("only documented pure helper calls are supported")
		}
	case *ast.IdentifierNode:
		if n.Value != "inputs" && !helperName(n.Value) {
			v.err = fmt.Errorf("unknown name %q; use inputs.name", n.Value)
		}
	case *ast.BinaryNode:
		switch n.Operator {
		case "+", "==", "!=", "and", "or", "&&", "||", "<", "<=", ">", ">=":
		default:
			v.err = fmt.Errorf("unsupported operator %q", n.Operator)
		}
	case *ast.UnaryNode:
		if n.Operator != "!" && n.Operator != "not" {
			v.err = fmt.Errorf("unsupported operator %q", n.Operator)
		}
	case *ast.StringNode, *ast.BoolNode, *ast.NilNode, *ast.ConditionalNode, *ast.ArrayNode:
	default:
		v.err = fmt.Errorf("unsupported expression construct %T", n)
	}
}
func helperName(name string) bool {
	switch name {
	case "quote", "flag", "boolFlag", "repeatFlag", "join", "default", "empty":
		return true
	}
	return false
}
func expressionEnv(inputs map[string]any) map[string]any {
	return map[string]any{"inputs": inputs, "quote": quote, "flag": flag, "boolFlag": boolFlag, "repeatFlag": expressionRepeatFlag, "join": expressionJoin, "default": defaultValue, "empty": empty}
}
func compileExpression(source string, names map[string]models.Input, boolean bool) (*vm.Program, []string, error) {
	if len(source) > 4096 {
		return nil, nil, fmt.Errorf("expression exceeds 4 KiB")
	}
	parseConfig := conf.CreateNew()
	expr.DisableAllBuiltins()(parseConfig)
	parseConfig.MaxNodes = 1000
	tree, err := parser.ParseWithConfig(source, parseConfig)
	if err != nil {
		return nil, nil, err
	}
	visitor := &expressionCheck{names: names, dependencies: map[string]bool{}}
	ast.Walk(&tree.Node, visitor)
	if visitor.err != nil {
		return nil, nil, visitor.err
	}
	kind, err := expressionType(tree.Node, names)
	if err != nil {
		return nil, nil, err
	}
	expected := "string"
	if boolean {
		expected = "boolean"
	}
	if kind != expected {
		return nil, nil, fmt.Errorf("expression must return %s, got %s", expected, kind)
	}
	samples := map[string]any{}
	for name, in := range names {
		samples[name] = in.Zero()
	}
	opts := []expr.Option{expr.Env(expressionEnv(samples)), expr.DisableAllBuiltins(), expr.MaxNodes(1000)}
	if boolean {
		opts = append(opts, expr.AsBool())
	} else {
		opts = append(opts, expr.AsKind(reflect.String))
	}
	program, err := expr.Compile(source, opts...)
	if err != nil {
		return nil, nil, err
	}
	dependencies := []string{}
	for name := range visitor.dependencies {
		dependencies = append(dependencies, name)
	}
	return program, dependencies, nil
}
