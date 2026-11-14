package templating

import (
	"github.com/samling/command-snippets/internal/models"
	"reflect"
	"strings"
	"testing"
)

func TestPartialPreviewUsesTypedFragmentsWithoutExposingCommand(t *testing.T) {
	snippet := models.Snippet{Name: "Partial typed values", Command: "echo {{{{literal}} {{flag}} {{choice}} {{items}} {{toggle}} {{hidden}} {{missing}}", Inputs: []models.Input{
		{Name: "flag", Kind: "flag", Flag: "-n", Default: "two words"},
		{Name: "choice", Kind: "choice", Default: "Memory", Choices: []models.Choice{{Label: "Memory", Value: "4"}}},
		{Name: "items", Kind: "repeat", Flag: "-e", Default: []string{"A=1", "B=two words"}},
		{Name: "toggle", Kind: "toggle", Flag: "-x", Default: true},
		{Name: "hidden", Default: "never shown", VisibleWhen: &models.Condition{Input: "toggle", Equals: false}, Special: map[string]string{"": "--leak"}},
		{Name: "missing", Required: true},
	}}
	compiled, err := Compile(snippet)
	if err != nil {
		t.Fatal(err)
	}
	result := compiled.Preview(nil)
	if result.Valid() || result.Command != "" {
		t.Fatal("partial command exposed")
	}
	var preview strings.Builder
	for _, part := range result.Preview {
		preview.WriteString(part.Text)
		if part.Text == "{{missing}}" && (!part.Unfilled || !part.Value) {
			t.Fatal("missing input not marked")
		}
	}
	want := "echo {{literal}} -n 'two words' 4 -e A=1 -e 'B=two words' -x  {{missing}}"
	if preview.String() != want {
		t.Fatalf("preview %q, want %q", preview.String(), want)
	}
	if command, err := compiled.Render(nil); err == nil || command != "" {
		t.Fatal("partial command rendered for execution")
	}
	complete := compiled.Preview(map[string]any{"missing": "done"})
	if !complete.Valid() || complete.Command != strings.Replace(want, "{{missing}}", "done", 1) {
		t.Fatalf("complete typed render failed: %+v", complete)
	}
}

func TestPartialPreviewEvaluatesExpressionsByTheirOwnInputValidity(t *testing.T) {
	snippet := models.Snippet{Name: "Docker run", Command: "docker run {{port_arg}} {{image_name_arg}} {{combined}} {{constant}}", Inputs: []models.Input{
		{Name: "port", Validate: &models.Validation{Range: []int{1, 65535}}},
		{Name: "image_name", Required: true},
	}, Expressions: map[string]string{
		"port_arg":       `inputs.port == "" ? "" : flag("-p", inputs.port + ":" + inputs.port)`,
		"image_name_arg": `inputs.image_name`,
		"combined":       `inputs.port + inputs.image_name`,
		"constant":       `"literal"`,
	}}
	compiled, err := Compile(snippet)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		values map[string]any
		want   string
	}{
		{"optional port before required image", map[string]any{"port": "8080"}, "docker run -p 8080:8080 {{image_name_arg}} {{combined}} literal"},
		{"invalid port with valid image", map[string]any{"port": "0", "image_name": "alpine"}, "docker run {{port_arg}} alpine {{combined}} literal"},
		{"wrong image type with valid port", map[string]any{"port": "8080", "image_name": 42}, "docker run -p 8080:8080 {{image_name_arg}} {{combined}} literal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := compiled.Preview(tc.values)
			var preview strings.Builder
			for _, part := range result.Preview {
				preview.WriteString(part.Text)
				if strings.HasPrefix(part.Text, "{{") && !part.Unfilled {
					t.Fatalf("unresolved expression is not marked unfilled: %+v", part)
				}
			}
			if preview.String() != tc.want {
				t.Fatalf("preview %q, want %q", preview.String(), tc.want)
			}
			if result.Valid() || result.Command != "" {
				t.Fatal("partial expression preview exposed executable command")
			}
			if command, err := compiled.Render(tc.values); err == nil || command != "" {
				t.Fatal("invalid expression inputs rendered a command")
			}
		})
	}
	complete := compiled.Preview(map[string]any{"port": "8080", "image_name": "alpine"})
	if !complete.Valid() || complete.Command != "docker run -p 8080:8080 alpine 8080alpine literal" {
		t.Fatalf("completed expression rendering changed: %+v", complete)
	}
}

func TestPreviewPartsNameTheirInputs(t *testing.T) {
	compiled, err := Compile(models.Snippet{Name: "Logs", Command: "awk {{awk_script}} {{logfile}} | sed {{sed_script}}", Inputs: []models.Input{{Name: "pattern", Required: true}, {Name: "extract", Default: "(.*)"}, {Name: "logfile", Required: true}}, Expressions: map[string]string{"awk_script": `quote("/" + inputs.pattern + "/")`, "sed_script": `quote(inputs.extract + inputs.pattern)`}})
	if err != nil {
		t.Fatal(err)
	}
	result := compiled.Preview(nil)
	got := map[string][2][]string{}
	for _, part := range result.Preview {
		if part.Value {
			got[part.Text] = [2][]string{part.Inputs, part.Missing}
		}
	}
	// Dependencies follow the snippet's input order; Missing lists only blocking inputs.
	if want := ([2][]string{{"pattern"}, {"pattern"}}); !reflect.DeepEqual(got["{{awk_script}}"], want) {
		t.Fatalf("awk_script parts = %v", got["{{awk_script}}"])
	}
	if want := ([2][]string{{"logfile"}, {"logfile"}}); !reflect.DeepEqual(got["{{logfile}}"], want) {
		t.Fatalf("logfile parts = %v", got["{{logfile}}"])
	}
	if want := ([2][]string{{"pattern", "extract"}, {"pattern"}}); !reflect.DeepEqual(got["{{sed_script}}"], [2][]string{{"extract", "pattern"}, {"pattern"}}) && !reflect.DeepEqual(got["{{sed_script}}"], want) {
		t.Fatalf("sed_script parts = %v", got["{{sed_script}}"])
	}
	if deps := got["{{sed_script}}"][0]; len(deps) != 2 || deps[0] != "pattern" {
		t.Fatalf("expression inputs not in snippet order: %v", deps)
	}
}
