package templating

import (
	"strings"
	"testing"

	"github.com/samling/command-snippets/internal/models"
)

func TestAdvancedHelpersAreTypedAndPure(t *testing.T) {
	s := models.Snippet{Name: "Forward service", Command: "{{resource}} {{ports}}", Inputs: []models.Input{{Name: "service"}, {Name: "host", Default: "8080"}, {Name: "target"}}, Expressions: map[string]string{"resource": `quote("svc/" + inputs.service)`, "ports": `quote(inputs.host + ":" + default(inputs.target, inputs.host))`}}
	c, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Render(map[string]any{"service": "api"})
	if err != nil || got != "'svc/api' '8080:8080'" {
		t.Fatalf("%q %v", got, err)
	}
	for _, code := range []string{`env("HOME")`, `len(inputs.service)`, `[1,2,3][0]`, `inputs.service + "x" + "y" + 3`, strings.Repeat(" ", 4097)} {
		bad := s
		bad.Expressions = map[string]string{"resource": code, "ports": `"ok"`}
		if _, err := Compile(bad); err == nil {
			t.Fatalf("unsupported expression accepted: %s", code)
		}
	}
}

func TestExpressionsEnforceInputAndHelperTypes(t *testing.T) {
	inputs := []models.Input{{Name: "text"}, {Name: "enabled", Kind: "toggle", Flag: "-x"}, {Name: "items", Kind: "repeat", Flag: "-e"}}
	for _, source := range []string{`quote(inputs.enabled)`, `inputs.enabled + "x"`, `inputs.text == inputs.enabled`, `empty(inputs.text) ? "x" : true`, `join([true], ",")`, `flag("-x", inputs.items)`} {
		if _, err := Compile(models.Snippet{Name: "Typed", Command: "{{out}}", Inputs: inputs, Expressions: map[string]string{"out": source}}); err == nil {
			t.Fatalf("untyped expression accepted: %s", source)
		}
	}
	c, err := Compile(models.Snippet{Name: "Helpers", Command: "{{out}}", Inputs: inputs, Expressions: map[string]string{"out": `join([flag("-n", inputs.text), boolFlag("-x", inputs.enabled)], " ")`}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Render(map[string]any{"text": "team-a", "enabled": true})
	if err != nil || got != "-n team-a -x" {
		t.Fatalf("helper composition %q: %v", got, err)
	}
}

func TestConditionsUseEffectiveValuesInDependencyOrder(t *testing.T) {
	s := models.Snippet{Name: "Dependency", Command: "echo {{child}} {{dependent}}", Inputs: []models.Input{
		{Name: "dependent", Default: "visible", VisibleWhen: &models.Condition{Expr: `inputs.child != ""`}},
		{Name: "child", Default: "secret", VisibleWhen: &models.Condition{Input: "mode", Equals: "show"}},
		{Name: "mode", Default: "hide"},
	}}
	c, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	result := c.Preview(nil)
	if !result.Valid() || result.Visible["dependent"] || result.Command != "echo  " {
		t.Fatalf("hidden dependency: %+v", result)
	}
	result = c.Preview(map[string]any{"mode": "show"})
	if !result.Valid() || !result.Visible["dependent"] || result.Command != "echo secret visible" {
		t.Fatalf("visible dependency: %+v", result)
	}
}

func TestPresetsKeepTypedValuesAndItemBoundaries(t *testing.T) {
	s := models.Snippet{Inputs: []models.Input{{Name: "env", Kind: "repeat"}, {Name: "enabled", Kind: "toggle"}, {Name: "text"}}}
	got, err := Presets(s, []string{"env=A=1", "env=B=hello world", "enabled=TRUE", "text="})
	if err != nil {
		t.Fatal(err)
	}
	if got["enabled"] != true || got["text"] != "" || strings.Join(got["env"].([]string), "|") != "A=1|B=hello world" {
		t.Fatalf("presets %+v", got)
	}
	for _, args := range [][]string{{"text=x", "text=y"}, {"enabled=yes"}, {"unknown=x"}, {"missing separator"}} {
		if _, err := Presets(s, args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
