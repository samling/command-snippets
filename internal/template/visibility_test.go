package template

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/samling/command-snippets/internal/models"
)

func TestNewFormModelSkipsInitiallyHiddenFields(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "all", Choices: []string{"all", "named"}},
			{Name: "namespace", VisibleIf: `mode == "named"`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	if len(model.fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(model.fields))
	}
	if model.fields[0].variable.Name != "mode" {
		t.Fatalf("got field %q, want mode", model.fields[0].variable.Name)
	}
}

func TestFormValuesIncludeHiddenVariableAsEmpty(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "all", Choices: []string{"all", "named"}},
			{Name: "namespace", VisibleIf: `mode == "named"`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	values := model.getValues()
	if values["namespace"] != "" {
		t.Fatalf("hidden namespace got %q, want empty", values["namespace"])
	}
}

func TestFormRefreshShowsDependentFieldAfterChoiceChange(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "all", Choices: []string{"all", "named"}},
			{Name: "namespace", VisibleIf: `mode == "named"`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	if len(model.fields) != 1 {
		t.Fatalf("got %d fields before refresh, want 1", len(model.fields))
	}
	model.fields[0].value = "named"
	model.refreshVisibleFields()
	if len(model.fields) != 2 {
		t.Fatalf("got %d fields after refresh, want 2", len(model.fields))
	}
	if model.fields[1].variable.Name != "namespace" {
		t.Fatalf("got dependent field %q, want namespace", model.fields[1].variable.Name)
	}
}

func TestFormChoicesAppearAsEnumOptions(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "all", Choices: []string{"all", "named"}},
		},
	}

	model := newFormModel(snippet, nil, nil)
	if len(model.fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(model.fields))
	}
	options := model.fields[0].enumOptions
	if len(options) != 2 || options[0] != "all" || options[1] != "named" {
		t.Fatalf("got enum options %#v, want [all named]", options)
	}
}

func TestFormEnterValidationIgnoresHiddenRequiredVariable(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "all", Choices: []string{"all", "named"}},
			{Name: "namespace", Required: true, VisibleIf: `mode == "named"`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	form, ok := updated.(formModel)
	if !ok {
		t.Fatalf("updated model has type %T, want formModel", updated)
	}
	if !form.done {
		t.Fatal("form was not submitted")
	}
	if len(form.fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(form.fields))
	}
	if form.fields[0].errorMessage != "" {
		t.Fatalf("visible field error got %q, want empty", form.fields[0].errorMessage)
	}
}

func TestFormRefreshRestoresInitiallyHiddenDefault(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "all", Choices: []string{"all", "named"}},
			{Name: "namespace", DefaultValue: "default", VisibleIf: `mode == "named"`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	model.fields[0].value = "named"
	model.refreshVisibleFields()

	if len(model.fields) != 2 {
		t.Fatalf("got %d fields, want 2", len(model.fields))
	}
	if model.fields[1].value != "default" {
		t.Fatalf("got restored value %q, want default", model.fields[1].value)
	}
}

func TestFormRefreshRestoresUserValueAfterHidingAndShowing(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "named", Choices: []string{"all", "named"}},
			{Name: "namespace", DefaultValue: "default", VisibleIf: `mode == "named"`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	if len(model.fields) != 2 {
		t.Fatalf("got %d fields initially, want 2", len(model.fields))
	}
	model.fields[1].value = "custom"
	model.fields[0].value = "all"
	model.refreshVisibleFields()
	if len(model.fields) != 1 {
		t.Fatalf("got %d fields after hiding, want 1", len(model.fields))
	}
	if values := model.getValues(); values["namespace"] != "" {
		t.Fatalf("hidden namespace submitted value got %q, want empty", values["namespace"])
	}

	model.fields[0].value = "named"
	model.refreshVisibleFields()
	if len(model.fields) != 2 {
		t.Fatalf("got %d fields after showing, want 2", len(model.fields))
	}
	if model.fields[1].value != "custom" {
		t.Fatalf("got restored value %q, want custom", model.fields[1].value)
	}
}

func TestFormRefreshSurfacesInvalidVisibleIf(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "all", Choices: []string{"all", "named"}},
			{Name: "namespace", VisibleIf: `mode ==`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	if model.visibilityError == "" {
		t.Fatal("got empty visibility error, want invalid visible_if error")
	}
	if !strings.Contains(model.visibilityError, "namespace") || !strings.Contains(model.visibilityError, "visible_if") {
		t.Fatalf("visibility error got %q, want variable name and visible_if", model.visibilityError)
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	form := updated.(formModel)
	if form.done {
		t.Fatal("form submitted despite invalid visible_if")
	}
}

func TestFormViewRendersVisibilityError(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", DefaultValue: "all", Choices: []string{"all", "named"}},
			{Name: "namespace", VisibleIf: `mode ==`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	view := model.View()
	errorText := strings.Split(model.visibilityError, "\n")[0]
	if !strings.Contains(view, errorText) {
		t.Fatalf("view %q does not contain visibility error text %q", view, errorText)
	}
}
