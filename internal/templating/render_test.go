package templating

import (
	"fmt"
	"strings"
	"testing"

	"github.com/samling/command-snippets/internal/models"
)

func TestRenderLimitBoundaryNeverReturnsPartialCommand(t *testing.T) {
	const limit = 1 << 20
	for _, expanded := range []bool{false, true} {
		for _, size := range []int{limit - 1, limit, limit + 1} {
			t.Run(fmt.Sprintf("expanded=%t/bytes=%d", expanded, size), func(t *testing.T) {
				want := strings.Repeat("x", size)
				snippet := models.Snippet{Name: "Boundary", Command: want}
				values := map[string]any{}
				if expanded {
					snippet.Command = "{{value}}"
					snippet.Inputs = []models.Input{{Name: "value"}}
					values["value"] = want
				}
				compiled, err := Compile(snippet)
				if err != nil {
					t.Fatal(err)
				}
				preview := compiled.Preview(values)
				command, err := compiled.Render(values)
				if size <= limit {
					if !preview.Valid() || err != nil || preview.Command != want || command != want {
						t.Fatalf("valid boundary rejected: %v", err)
					}
				} else {
					if preview.Valid() || preview.Command != "" || command != "" || err == nil || preview.Errors["command"] != "rendered command exceeds 1 MiB" {
						t.Fatalf("limit not fail-closed: preview=%+v error=%v", preview, err)
					}
					second := compiled.Preview(values)
					if second.Error().Error() != preview.Error().Error() {
						t.Fatal("limit error not deterministic")
					}
				}
			})
		}
	}
}

func TestRenderGuidedFlagsChoicesAndLiteralShell(t *testing.T) {
	s := models.Snippet{Name: "Pod resource usage, sorted", Command: "kubectl top pods {{namespace}} | sort -k{{sort_by}}; echo \"$header ${HOME}\"\n", Inputs: []models.Input{
		{Name: "namespace", Kind: "flag", Flag: "-n", Special: map[string]string{"all": "-A"}},
		{Name: "sort_by", Kind: "choice", Choices: []models.Choice{{Label: "CPU", Value: "3"}, {Label: "Memory", Value: "4"}}, Default: "CPU"},
	}}
	c, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ namespace, sort, want string }{{"", "CPU", " | sort -k3"}, {"team-a", "Memory", "-n team-a | sort -k4"}, {"all", "Memory", "-A | sort -k4"}} {
		got, err := c.Render(map[string]any{"namespace": tc.namespace, "sort_by": tc.sort})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, tc.want) || !strings.HasSuffix(got, "echo \"$header ${HOME}\"\n") {
			t.Fatalf("render: %q", got)
		}
	}
}
func TestRenderInvalidPresetsAndHiddenValues(t *testing.T) {
	s := models.Snippet{Name: "Conditional", Command: "echo {{port}} {{namespace}}", Inputs: []models.Input{
		{Name: "mode", Kind: "choice", Choices: []models.Choice{{Label: "Default", Value: ""}, {Label: "Named", Value: ""}}},
		{Name: "namespace", Required: true, Default: "secret", VisibleWhen: &models.Condition{Input: "mode", Equals: "Named"}, Special: map[string]string{"": "--leak"}},
		{Name: "port", Default: "8080", Validate: &models.Validation{Range: []int{1, 65535}}},
	}}
	c, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Render(map[string]any{"port": "80"})
	if err != nil || got != "echo 80 " {
		t.Fatalf("hidden output %q: %v", got, err)
	}
	if _, err := c.Render(map[string]any{"port": "0"}); err == nil {
		t.Fatal("all preset invalid range accepted")
	}
	if _, err := c.Render(map[string]any{"port": "", "mode": "Named", "namespace": ""}); err == nil {
		t.Fatal("visible required value accepted")
	}
	if _, err := c.Render(map[string]any{"mode": "missing"}); err == nil {
		t.Fatal("unknown choice accepted")
	}
	if _, err := c.Render(map[string]any{"unknown": "x"}); err == nil {
		t.Fatal("unknown preset accepted")
	}
}
func TestRenderRepeatQuotingAndExpressions(t *testing.T) {
	s := models.Snippet{Name: "Container", Command: "echo {{env}} {{resource}} $HOME {{text}}", Inputs: []models.Input{{Name: "env", Kind: "repeat", Flag: "-e"}, {Name: "service"}, {Name: "text"}}, Expressions: map[string]string{"resource": `quote("svc/" + inputs.service)`}}
	c, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Render(map[string]any{"env": []string{"A=1", "B=hello world"}, "service": "o'neil", "text": "{{service}}"})
	if err != nil {
		t.Fatal(err)
	}
	want := `echo -e A=1 -e 'B=hello world' 'svc/o'\''neil' $HOME '{{service}}'`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
func TestCompileRejectsInvalidLanguage(t *testing.T) {
	for _, command := range []string{"echo {{missing}}", "echo {{x", "echo {{x + 1}}"} {
		if _, err := Compile(models.Snippet{Name: "Bad", Command: command}); err == nil {
			t.Fatalf("accepted %q", command)
		}
	}
	for _, expression := range []string{"inputs.missing", "42", `map(1..10, {#})`, `inputs.x.String()`} {
		_, err := Compile(models.Snippet{Name: "Bad", Command: "{{out}}", Inputs: []models.Input{{Name: "x"}}, Expressions: map[string]string{"out": expression}})
		if err == nil {
			t.Fatalf("accepted expression %s", expression)
		}
	}
	s := models.Snippet{Name: "Cycle", Command: "echo ok", Inputs: []models.Input{{Name: "a", VisibleWhen: &models.Condition{Input: "b", Equals: ""}}, {Name: "b", VisibleWhen: &models.Condition{Input: "a", Equals: ""}}}}
	if _, err := Compile(s); err == nil {
		t.Fatal("condition cycle accepted")
	}
	c, err := Compile(models.Snippet{Name: "Literal", Command: "echo {{{{example}} ${HOME}"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Render(nil)
	if err != nil || got != "echo {{example}} ${HOME}" {
		t.Fatalf("literal %q %v", got, err)
	}
}
