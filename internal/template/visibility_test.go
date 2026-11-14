package template

import (
	"github.com/samling/command-snippets/internal/models"
	"testing"
)

func TestFormHiddenValuesRestoreWithoutEmission(t *testing.T) {
	s := models.Snippet{Name: "Namespace", Command: "echo {{namespace}}", Inputs: []models.Input{{Name: "mode", Kind: "choice", Choices: []models.Choice{{Label: "Default", Value: ""}, {Label: "Named", Value: ""}}}, {Name: "namespace", Required: true, Default: "cached", VisibleWhen: &models.Condition{Input: "mode", Equals: "Named"}}}}
	f, err := NewForm(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Controls) != 1 || f.Result.Command != "echo " {
		t.Fatal("initial hidden default emitted")
	}
	*f.Choices["mode"] = "Named"
	f.Refresh()
	f.Texts["namespace"].Set("team-a")
	f.Refresh()
	f.Focus = 1
	*f.Choices["mode"] = "Default"
	f.Refresh()
	if f.Focus != 0 || f.Result.Command != "echo " {
		t.Fatal("hidden focus/output incorrect")
	}
	*f.Choices["mode"] = "Named"
	f.Refresh()
	if f.Texts["namespace"].String() != "team-a" || f.Result.Command != "echo team-a" {
		t.Fatal("cache did not restore")
	}
}
func TestFormRepeatItemsRemainSeparate(t *testing.T) {
	s := models.Snippet{Name: "Env", Command: "echo {{env}}", Inputs: []models.Input{{Name: "env", Kind: "repeat", Flag: "-e", Default: []string{"A=1", "B=hello world"}}}}
	f, err := NewForm(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.Result.Command != "echo -e A=1 -e 'B=hello world'" {
		t.Fatalf("repeat preview %q", f.Result.Command)
	}
}
