package templating

import (
	"strings"
	"testing"

	"github.com/samling/command-snippets/internal/models"
)

func TestToggleRejectsContentChecksButSupportsRequiredConditions(t *testing.T) {
	for _, validation := range []*models.Validation{{Pattern: "true"}, {Range: []int{0, 1}}, {Regex: true}} {
		_, err := Compile(models.Snippet{Name: "Invalid toggle", Command: "echo {{enabled}}", Inputs: []models.Input{{Name: "enabled", Kind: "toggle", Flag: "-x", Validate: validation}}})
		if err == nil || !strings.Contains(err.Error(), "enabled") {
			t.Fatal("toggle content check silently accepted")
		}
	}
	s := models.Snippet{Name: "Required toggle", Command: "echo {{enabled}}", Inputs: []models.Input{{Name: "mode", Default: "required"}, {Name: "enabled", Kind: "toggle", Flag: "-x", RequiredWhen: &models.Condition{Input: "mode", Equals: "required"}}}}
	c, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Render(nil); err == nil {
		t.Fatal("false required toggle accepted")
	}
	got, err := c.Render(map[string]any{"enabled": true})
	if err != nil || got != "echo -x" {
		t.Fatalf("toggle %q %v", got, err)
	}
}
func TestTypedHiddenPresetsAndEmptyLiteralLists(t *testing.T) {
	s := models.Snippet{Name: "Hidden", Command: "echo {{out}}", Inputs: []models.Input{{Name: "mode", Default: "hide"}, {Name: "enabled", Kind: "toggle", Flag: "-x", VisibleWhen: &models.Condition{Input: "mode", Equals: "show"}}}, Expressions: map[string]string{"out": `empty([]) ? "empty" : "not empty"`}}
	c, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Render(nil)
	if err != nil || got != "echo empty" {
		t.Fatalf("list literal %q: %v", got, err)
	}
	if _, err := c.Render(map[string]any{"enabled": "true"}); err == nil {
		t.Fatal("wrong structural preset type ignored when hidden")
	}
}
func BenchmarkPreview(b *testing.B) {
	s := models.Snippet{Name: "Preview", Command: "echo {{value}}", Inputs: []models.Input{{Name: "value", Default: "hello"}}}
	compiled, err := Compile(s)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.Preview(nil)
	}
}
