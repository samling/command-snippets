package templating

import (
	"strings"
	"testing"
)

func TestHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "flag empty", got: flag("-n", ""), want: ""},
		{name: "flag value", got: flag("-n", "default"), want: "-n default"},
		{name: "bool flag true", got: boolFlag("--verbose", true), want: "--verbose"},
		{name: "bool flag false", got: boolFlag("--verbose", false), want: ""},
		{name: "repeat flag empty", got: repeatFlag("-e", ""), want: ""},
		{name: "repeat flag one value", got: repeatFlag("-e", "TEST=TEST"), want: "-e TEST=TEST"},
		{name: "repeat flag two values", got: repeatFlag("-e", "TEST=TEST FOO=BAR"), want: "-e TEST=TEST -e FOO=BAR"},
		{name: "repeat flag quotes unsafe values", got: repeatFlag("--label", "name=hello;world env=prod"), want: "--label 'name=hello;world' --label env=prod"},
		{name: "default empty", got: defaultValue("", "fallback"), want: "fallback"},
		{name: "default value", got: defaultValue("actual", "fallback"), want: "actual"},
		{name: "join skips empty", got: joinNonEmpty([]string{"kubectl", "", "pods"}, " "), want: "kubectl pods"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestEmptyHelper(t *testing.T) {
	if !empty("") {
		t.Fatal("empty string should be empty")
	}
	if empty("value") {
		t.Fatal("non-empty string should not be empty")
	}
}

func TestQuoteShellEscapesSensitiveValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "whitespace", value: "hello world", want: "'hello world'"},
		{name: "dollar", value: "cost $5", want: "'cost $5'"},
		{name: "backticks", value: "run `date`", want: "'run `date`'"},
		{name: "single quote", value: "don't", want: "'don'\\''t'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quote(tt.value); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlagQuotesUnsafeValues(t *testing.T) {
	got := flag("--name", "prod; rm -rf /")
	if got == "--name prod; rm -rf /" {
		t.Fatal("flag should quote unsafe shell values")
	}
	if got != "--name 'prod; rm -rf /'" {
		t.Fatalf("got %q, want shell-quoted value", got)
	}
}

func TestEvalBool(t *testing.T) {
	ctx := map[string]string{"mode": "advanced", "output": "json"}
	got, err := EvalBool(`mode == "advanced" && output in ["json", "yaml"]`, ctx)
	if err != nil {
		t.Fatalf("EvalBool returned error: %v", err)
	}
	if !got {
		t.Fatal("expected expression to evaluate true")
	}
}

func TestInterpolate(t *testing.T) {
	ctx := map[string]string{"pattern": "hello world", "file": "app.log"}
	got, err := Interpolate(`grep ${quote(pattern)} ${file}`, ctx)
	if err != nil {
		t.Fatalf("Interpolate returned error: %v", err)
	}
	want := `grep 'hello world' app.log`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestInterpolateBraceInsideQuotedExpression(t *testing.T) {
	ctx := map[string]string{"pattern": ""}
	got, err := Interpolate(`x ${default(pattern, "}")} y`, ctx)
	if err != nil {
		t.Fatalf("Interpolate returned error: %v", err)
	}
	want := `x } y`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestInterpolateUnknownValue(t *testing.T) {
	_, err := Interpolate(`kubectl get pods ${namespace_arg}`, map[string]string{})
	if err == nil {
		t.Fatal("expected unknown value error")
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "plain", command: "docker run  nginx   --rm", want: "docker run nginx --rm"},
		{name: "single quotes", command: "printf 'a  b'  |  cat", want: "printf 'a  b' | cat"},
		{name: "double quotes", command: `echo "a  b"   done`, want: `echo "a  b" done`},
		{name: "escaped space", command: `echo a\ b   done`, want: `echo a\ b done`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeCommandWhitespace(tt.command)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveComputedValue(t *testing.T) {
	computed := map[string]ComputedValue{
		"output_arg": {Value: `flag("-o", output)`},
	}
	values := map[string]string{"output": "json"}
	got, err := ResolveComputed(computed, values)
	if err != nil {
		t.Fatalf("ResolveComputed returned error: %v", err)
	}
	if got["output_arg"] != "-o json" {
		t.Fatalf("got %q, want %q", got["output_arg"], "-o json")
	}
}

func TestResolveComputedValueRequiresExpression(t *testing.T) {
	computed := map[string]ComputedValue{
		"namespace_arg": {Value: "-A"},
	}
	_, err := ResolveComputed(computed, nil)
	if err == nil {
		t.Fatal("expected unquoted literal expression error")
	}
	if !strings.Contains(err.Error(), "computed namespace_arg value") {
		t.Fatalf("error %q should identify computed value", err)
	}
}

func TestResolveComputedSortedDependencies(t *testing.T) {
	computed := map[string]ComputedValue{
		"b_arg": {Value: `flag("--b", a_arg)`},
		"a_arg": {Value: `"alpha"`},
	}
	got, err := ResolveComputed(computed, nil)
	if err != nil {
		t.Fatalf("ResolveComputed returned error: %v", err)
	}
	if got["b_arg"] != "--b alpha" {
		t.Fatalf("got %q, want %q", got["b_arg"], "--b alpha")
	}
}

func TestResolveComputedLaterDependencyErrors(t *testing.T) {
	computed := map[string]ComputedValue{
		"a_arg": {Value: `flag("--a", b_arg)`},
		"b_arg": {Value: `"beta"`},
	}
	_, err := ResolveComputed(computed, nil)
	if err == nil {
		t.Fatal("expected later dependency error")
	}
	if !strings.Contains(err.Error(), "computed a_arg value") || !strings.Contains(err.Error(), "b_arg") {
		t.Fatalf("error %q should identify computed value and unavailable dependency", err)
	}
}

func TestResolveComputedCases(t *testing.T) {
	computed := map[string]ComputedValue{
		"namespace_arg": {
			Cases: []ComputedCase{
				{When: `namespace_mode == "all"`, Value: "-A"},
				{When: `namespace_mode == "named"`, Value: `${flag("-n", namespace)}`},
				{Default: true, Value: ""},
			},
		},
	}
	values := map[string]string{"namespace_mode": "named", "namespace": "default"}
	got, err := ResolveComputed(computed, values)
	if err != nil {
		t.Fatalf("ResolveComputed returned error: %v", err)
	}
	if got["namespace_arg"] != "-n default" {
		t.Fatalf("got %q, want %q", got["namespace_arg"], "-n default")
	}
}

func TestResolveComputedDefaultCase(t *testing.T) {
	computed := map[string]ComputedValue{
		"namespace_arg": {
			Cases: []ComputedCase{
				{When: `namespace_mode == "all"`, Value: "-A"},
				{Default: true, Value: `${flag("-n", namespace)}`},
			},
		},
	}
	values := map[string]string{"namespace_mode": "named", "namespace": "default"}
	got, err := ResolveComputed(computed, values)
	if err != nil {
		t.Fatalf("ResolveComputed returned error: %v", err)
	}
	if got["namespace_arg"] != "-n default" {
		t.Fatalf("got %q, want %q", got["namespace_arg"], "-n default")
	}
}

func TestResolveComputedCaseNonBoolWhen(t *testing.T) {
	computed := map[string]ComputedValue{
		"namespace_arg": {
			Cases: []ComputedCase{{When: `namespace_mode`, Value: "-A"}},
		},
	}
	values := map[string]string{"namespace_mode": "named"}
	_, err := ResolveComputed(computed, values)
	if err == nil {
		t.Fatal("expected non-bool when error")
	}
	if !strings.Contains(err.Error(), "computed namespace_arg case 0") || !strings.Contains(err.Error(), "boolean") {
		t.Fatalf("error %q should identify computed value, case index, and boolean requirement", err)
	}
}
