package template

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

func TestFormChoicesRenderEmptyOptionAsNone(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "output", Choices: []string{"", "wide", "yaml", "json"}},
		},
	}

	model := newFormModel(snippet, nil, nil)
	view := model.View()

	if !strings.Contains(view, "<none>") {
		t.Fatalf("view did not render empty choice as none:\n%s", view)
	}
	if strings.Contains(view, "<>") {
		t.Fatalf("view rendered empty choice as angle-only placeholder:\n%s", view)
	}
	if got := model.fields[0].value; got != "" {
		t.Fatalf("empty choice changed underlying value to %q", got)
	}
}

func TestFormChoicesUseEmptyLabelForEmptyOption(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "output", Choices: []string{"", "wide", "yaml", "json"}, EmptyLabel: "normal"},
		},
	}

	model := newFormModel(snippet, nil, nil)
	view := model.View()

	if !strings.Contains(view, "<normal>") {
		t.Fatalf("view did not render empty choice with custom label:\n%s", view)
	}
	if strings.Contains(view, "<none>") || strings.Contains(view, "<>") {
		t.Fatalf("view rendered fallback empty label instead of custom label:\n%s", view)
	}
	if got := model.fields[0].value; got != "" {
		t.Fatalf("empty choice changed underlying value to %q", got)
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

func TestNewFormModelInitialVisibilityUsesFirstChoiceWhenDefaultEmpty(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "mode", Choices: []string{"named", "all"}},
			{Name: "namespace", VisibleIf: `mode == "named"`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	if len(model.fields) != 2 {
		t.Fatalf("got %d fields, want 2", len(model.fields))
	}
	if model.fields[0].value != "named" {
		t.Fatalf("got mode value %q, want named", model.fields[0].value)
	}
	if model.fields[1].variable.Name != "namespace" {
		t.Fatalf("got dependent field %q, want namespace", model.fields[1].variable.Name)
	}
}

func TestNewFormModelInitialVisibilityUsesNormalizedBooleanDefault(t *testing.T) {
	snippet := &models.Snippet{
		Variables: []models.Variable{
			{Name: "enabled", Type: models.VarTypeBoolean},
			{Name: "name", VisibleIf: `enabled == "false"`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	if len(model.fields) != 2 {
		t.Fatalf("got %d fields, want 2", len(model.fields))
	}
	if model.fields[0].value != "false" {
		t.Fatalf("got enabled value %q, want false", model.fields[0].value)
	}
	if model.fields[1].variable.Name != "name" {
		t.Fatalf("got dependent field %q, want name", model.fields[1].variable.Name)
	}
}

func TestRenderCommandPreviewStylesComputedInterpolation(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	snippet := &models.Snippet{
		Command: "kubectl get pods ${namespace_arg}",
		Variables: []models.Variable{
			{Name: "namespace_mode", DefaultValue: "named", Choices: []string{"none", "all", "named"}},
			{Name: "namespace", DefaultValue: "default", VisibleIf: `namespace_mode == "named"`},
		},
		Computed: map[string]models.ComputedValue{
			"namespace_arg": {
				Cases: []models.ComputedCase{
					{When: `namespace_mode == "all"`, Value: "-A"},
					{When: `namespace_mode == "named"`, Value: `${flag("-n", namespace)}`},
					{Default: true},
				},
			},
		},
	}

	model := newFormModel(snippet, nil, nil)
	preview := model.renderCommandPreview()
	styledValue := filledVarStyle.Render("-n default")
	if !strings.Contains(preview, styledValue) {
		t.Fatalf("preview %q does not contain styled computed value %q", preview, styledValue)
	}
	if strings.Contains(preview, "${namespace_arg}") {
		t.Fatalf("preview still contains raw computed placeholder: %q", preview)
	}
}

func TestRenderCommandPreviewIncludesTemplateName(t *testing.T) {
	snippet := &models.Snippet{
		Name:    "docker-run-advanced",
		Command: "docker run ${image_arg}",
		Variables: []models.Variable{
			{Name: "image", DefaultValue: "nginx"},
		},
		Computed: map[string]models.ComputedValue{
			"image_arg": {Value: "image"},
		},
	}

	model := newFormModel(snippet, nil, nil)
	preview := model.renderCommandPreview()

	if !strings.Contains(preview, "Template: docker-run-advanced") {
		t.Fatalf("preview did not include template name:\n%s", preview)
	}
}

func TestRenderCommandPreviewNormalizesEmptyComputedSpacing(t *testing.T) {
	snippet := &models.Snippet{
		Command: "cmd ${empty_arg} ${filled_arg}",
		Variables: []models.Variable{
			{Name: "selector", DefaultValue: "test"},
		},
		Computed: map[string]models.ComputedValue{
			"empty_arg":  {Value: `""`},
			"filled_arg": {Value: `flag("--selector", selector)`},
		},
	}

	model := newFormModel(snippet, nil, nil)
	preview := model.renderCommandPreview()

	if !strings.Contains(preview, "cmd --selector test") {
		t.Fatalf("preview did not normalize empty computed placeholders:\n%s", preview)
	}
	if strings.Contains(preview, "cmd  --selector") {
		t.Fatalf("preview contains extra spaces from empty computed placeholders:\n%s", preview)
	}
}
