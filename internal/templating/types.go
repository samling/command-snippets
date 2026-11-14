package templating

import (
	"fmt"

	"github.com/expr-lang/expr/ast"
	"github.com/samling/command-snippets/internal/models"
)

// Expr map environments are dynamically typed. Check the fixed public input/
// helper language before handing it to Expr so schema errors cannot wait until use.
func expressionType(node ast.Node, inputs map[string]models.Input) (string, error) {
	fail := func(message string) (string, error) { return "", fmt.Errorf("%s", message) }
	switch n := node.(type) {
	case *ast.StringNode:
		return "string", nil
	case *ast.BoolNode:
		return "boolean", nil
	case *ast.MemberNode:
		root, ok := n.Node.(*ast.IdentifierNode)
		property, pok := n.Property.(*ast.StringNode)
		if !ok || !pok || root.Value != "inputs" {
			return fail("only declared input access is supported")
		}
		input, ok := inputs[property.Value]
		if !ok {
			return fail("unknown input " + property.Value)
		}
		switch input.InputKind() {
		case "toggle":
			return "boolean", nil
		case "repeat":
			return "strings", nil
		default:
			return "string", nil
		}
	case *ast.ArrayNode:
		for _, item := range n.Nodes {
			kind, err := expressionType(item, inputs)
			if err != nil {
				return "", err
			}
			if kind != "string" {
				return fail("literal lists must contain strings")
			}
		}
		return "strings", nil
	case *ast.UnaryNode:
		kind, err := expressionType(n.Node, inputs)
		if err != nil {
			return "", err
		}
		if kind != "boolean" {
			return fail("boolean negation needs a boolean")
		}
		return "boolean", nil
	case *ast.BinaryNode:
		left, err := expressionType(n.Left, inputs)
		if err != nil {
			return "", err
		}
		right, err := expressionType(n.Right, inputs)
		if err != nil {
			return "", err
		}
		switch n.Operator {
		case "+":
			if left != "string" || right != "string" {
				return fail("string concatenation needs two strings")
			}
			return "string", nil
		case "and", "or", "&&", "||":
			if left != "boolean" || right != "boolean" {
				return fail("boolean operators need two booleans")
			}
			return "boolean", nil
		case "==", "!=":
			if left != right {
				return fail("comparison operands must have the same type")
			}
			return "boolean", nil
		case "<", "<=", ">", ">=":
			if left != "string" || right != "string" {
				return fail("ordered comparison needs strings")
			}
			return "boolean", nil
		}
		return fail("unsupported operator " + n.Operator)
	case *ast.ConditionalNode:
		condition, err := expressionType(n.Cond, inputs)
		if err != nil {
			return "", err
		}
		if condition != "boolean" {
			return fail("ternary condition needs a boolean")
		}
		left, err := expressionType(n.Exp1, inputs)
		if err != nil {
			return "", err
		}
		right, err := expressionType(n.Exp2, inputs)
		if err != nil {
			return "", err
		}
		if left != right {
			return fail("ternary branches must have the same type")
		}
		return left, nil
	case *ast.CallNode:
		callee, ok := n.Callee.(*ast.IdentifierNode)
		if !ok {
			return fail("method calls are unsupported")
		}
		var expected []string
		result := "string"
		switch callee.Value {
		case "quote":
			expected = []string{"string"}
		case "flag", "default":
			expected = []string{"string", "string"}
		case "boolFlag":
			expected = []string{"string", "boolean"}
		case "repeatFlag":
			expected = []string{"string", "strings"}
		case "join":
			expected = []string{"strings", "string"}
		case "empty":
			expected = []string{"any"}
			result = "boolean"
		default:
			return fail("unknown helper " + callee.Value)
		}
		if len(n.Arguments) != len(expected) {
			return fail(callee.Value + ": wrong argument count")
		}
		for i, arg := range n.Arguments {
			kind, err := expressionType(arg, inputs)
			if err != nil {
				return "", err
			}
			if expected[i] != "any" && kind != expected[i] {
				return fail(fmt.Sprintf("%s argument %d needs %s, got %s", callee.Value, i+1, expected[i], kind))
			}
		}
		return result, nil
	}
	return fail(fmt.Sprintf("unsupported expression value %T", node))
}
