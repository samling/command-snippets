package templating

import "testing"

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

func TestInterpolateUnknownValue(t *testing.T) {
	_, err := Interpolate(`kubectl get pods ${namespace_arg}`, map[string]string{})
	if err == nil {
		t.Fatal("expected unknown value error")
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	got := NormalizeCommandWhitespace("docker run  nginx   --rm")
	want := "docker run nginx --rm"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
