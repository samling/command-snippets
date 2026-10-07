package internal_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samling/command-snippets/internal/cmd"
)

func TestRenderLimitFailureHasEmptyStdout(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "config.yaml")
	for _, expanded := range []bool{false, true} {
		body := "snippets:\n  - name: Large\n    command: " + strings.Repeat("x", (1<<20)+1) + "\n"
		args := []string{"render", "Large"}
		if expanded {
			body = "snippets:\n  - name: Large\n    command: '{{value}}'\n    inputs:\n      - name: value\n"
			args = append(args, "--set", "value="+strings.Repeat("x", (1<<20)+1))
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		root := cmd.NewRoot()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs(append([]string{"--config", path}, args...))
		err := root.Execute()
		if err == nil || stdout.Len() != 0 || !strings.Contains(err.Error(), "rendered command exceeds 1 MiB") {
			t.Fatalf("expanded=%t error=%v stdout length=%d", expanded, err, stdout.Len())
		}
	}
}

func TestCanonicalCLIIntegrationAndOutputSafety(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "config.yaml")
	data := `settings:
  project_source: false
snippets:
  - name: Friendly command
    command: echo {{value}} $HOME
    inputs:
      - name: value
        required: true
  - name: Duplicate
    command: echo one
  - name: Duplicate
    command: echo two
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
		fail bool
	}{{[]string{"render", "Friendly command", "--set", "value=hello world"}, "echo 'hello world' $HOME\n", false}, {[]string{"render", "Friendly command"}, "", true}, {[]string{"render", "Duplicate"}, "", true}, {[]string{"render", "Friendly command", "--set", "unknown=x"}, "", true}, {[]string{"validate"}, "", false}, {[]string{"list", "--query", "Friendly"}, "Friendly command\n", false}, {[]string{"list"}, "Duplicate\nDuplicate\nFriendly command\n", false}, {[]string{"list", "--query", "no-match-ever"}, "", false}, {[]string{"exec", "Friendly command", "--set", "value=hello", "--run", "--prompt"}, "", true}} {
		root := cmd.NewRoot()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs(append([]string{"--config", path}, tc.args...))
		err := root.Execute()
		if (err != nil) != tc.fail || stdout.String() != tc.want {
			t.Fatalf("%v stdout=%q error=%v", tc.args, stdout.String(), err)
		}
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != data {
		t.Fatal("read-only CLI changed config")
	}
	root := cmd.NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--config", path, "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	wantJSON := "[{\"name\":\"Duplicate\",\"description\":\"\",\"tags\":[],\"source\":%q,\"id\":null},{\"name\":\"Duplicate\",\"description\":\"\",\"tags\":[],\"source\":%q,\"id\":null},{\"name\":\"Friendly command\",\"description\":\"\",\"tags\":[],\"source\":%q,\"id\":null}]\n"
	if out.String() != fmt.Sprintf(wantJSON, path, path, path) {
		t.Fatalf("JSON list changed: %q", out.String())
	}
}

func TestCanonicalCLITypedPresetsIDsAndExplicitExecution(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "config.yaml")
	id := "12345678-1234-4234-8234-123456789abc"
	body := `settings: {project_source: false, sources: ['missing*.yaml']}
snippets:
  - id: ` + id + `
    name: Duplicate
    command: echo first
  - name: Duplicate
    command: echo second
  - name: Typed
    command: echo {{port}} {{choice}} {{toggle}} {{pattern}} {{env}}
    inputs:
      - {name: port, default: "80", validate: {range: [1, 65535]}}
      - {name: choice, kind: choice, choices: [{label: CPU, value: "3"}, {label: Memory, value: "4"}]}
      - {name: toggle, kind: toggle, flag: -x}
      - {name: pattern, validate: {regex: true}}
      - {name: env, kind: repeat, flag: -e}
  - name: Safe command
    command: echo safe
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	execute := func(args ...string) (string, string, error) {
		t.Helper()
		root := cmd.NewRoot()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs(append([]string{"--config", path}, args...))
		err := root.Execute()
		return stdout.String(), stderr.String(), err
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
		fail bool
	}{
		{"ID lookup", []string{"render", "--id", id}, "echo first\n", false},
		{"trimmed friendly lookup", []string{"render", " Safe command "}, "echo safe\n", false},
		{"duplicate exec", []string{"exec", "Duplicate"}, "", true},
		{"malformed ID", []string{"render", "--id", "slug"}, "", true},
		{"missing ID", []string{"render", "--id", "87654321-1234-4234-8234-123456789abc"}, "", true},
		{"NAME plus ID", []string{"render", "Duplicate", "--id", id}, "", true},
		{"missing selection", []string{"render"}, "", true},
		{"invalid range", []string{"render", "Typed", "--set", "port=0"}, "", true},
		{"invalid choice", []string{"render", "Typed", "--set", "choice=missing"}, "", true},
		{"invalid toggle", []string{"render", "Typed", "--set", "toggle=yes"}, "", true},
		{"invalid regex", []string{"render", "Typed", "--set", "pattern=["}, "", true},
		{"duplicate scalar", []string{"render", "Typed", "--set", "port=80", "--set", "port=90"}, "", true},
		{"unknown preset", []string{"render", "Typed", "--set", "unknown=x"}, "", true},
		{"repeat and equals", []string{"render", "Typed", "--set", "toggle=TRUE", "--set", "env=A=1", "--set", "env=B=hello world"}, "echo 80 3 -x  -e A=1 -e 'B=hello world'\n", false},
		{"explicit empty does not restore default", []string{"render", "Typed", "--set", "port="}, "echo  3   \n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := execute(tc.args...)
			if (err != nil) != tc.fail || stdout != tc.want {
				t.Fatalf("stdout=%q want=%q error=%v", stdout, tc.want, err)
			}
			if !strings.Contains(stderr, "Warning:") {
				t.Fatal("unmatched glob warning missing from diagnostic writer")
			}
		})
	}
	// Replace only this test's PATH with a benign shell receipt. Authored
	// command text is captured as an argument, never evaluated by the stub.
	marker := filepath.Join(dir, "shell-called")
	shell := "#!/bin/sh\nprintf '%s\\n' \"$*\" > '" + marker + "'\nprintf 'stub executed\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "sh"), []byte(shell), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	for i, args := range [][]string{{"render", "Safe command"}, {"exec", "Safe command"}, {"exec", "Typed", "--run", "--set", "port=0", "--set", "choice=CPU", "--set", "toggle=false", "--set", "pattern=", "--set", "env=A=1"}} {
		stdout, _, err := execute(args...)
		if i < 2 && (err != nil || stdout != "echo safe\n") {
			t.Fatalf("default path failed: stdout=%q error=%v", stdout, err)
		}
		if i == 2 && (err == nil || stdout != "") {
			t.Fatalf("invalid execution was accepted: stdout=%q error=%v", stdout, err)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("default/invalid path spawned a shell")
		}
	}
	stdout, _, err := execute("exec", "Safe command", "--run")
	if err != nil || stdout != "stub executed\n" {
		t.Fatalf("explicit execution: stdout=%q error=%v", stdout, err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "-c echo safe\n" {
		t.Fatalf("shell arguments: %q %v", data, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != body {
		t.Fatal("CLI verification changed source")
	}
}

func TestCLIHelpAndGenerationIgnoreInvalidLibrary(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("invalid: ["), 0600); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"--help", "--version", "--generate-config"} {
		root := cmd.NewRoot()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"--config", path, arg})
		if err := root.Execute(); err != nil || out.Len() == 0 {
			t.Fatalf("%s requires valid library: %v %q", arg, err, out.String())
		}
	}
}
