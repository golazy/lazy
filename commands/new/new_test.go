package newcommand

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/golazy/lazy/commands"
)

func readVersion(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

type invocation struct {
	command string
	args    []string
	options commands.Options
}

func TestRejectsInvalidModuleName(t *testing.T) {
	err := (Command{Version: readVersion(t), Stdout: &bytes.Buffer{}}).Execute("../my_app")
	if err == nil || !strings.Contains(err.Error(), "invalid module name") {
		t.Fatalf("error = %v", err)
	}
}

func TestClonesRenamesAndValidates(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	var calls []invocation

	command := Command{
		Version: readVersion(t),
		Dir:     dir,
		Stdout:  &stdout,
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			if name != "git" {
				return nil
			}

			destination := args[len(args)-1]
			if err := os.MkdirAll(filepath.Join(destination, ".git"), 0o755); err != nil {
				return err
			}
			writeFile(t, filepath.Join(destination, "go.mod"), "module sample_app\n")
			writeFile(
				t,
				filepath.Join(destination, "main.go"),
				"package main\nimport \"sample_app/app\"\n",
			)
			return nil
		},
	}

	if err := command.Execute("github.com/guillermo/my_app"); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(dir, "my_app")
	if _, err := os.Stat(filepath.Join(destination, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git still exists: %v", err)
	}
	assertFileContains(t, filepath.Join(destination, "go.mod"), "module github.com/guillermo/my_app")
	assertFileContains(t, filepath.Join(destination, "main.go"), `"github.com/guillermo/my_app/app"`)

	wantOutput := strings.Join([]string{
		"* Initializing the core app",
		"* Naming the app",
		"* Validating",
		"Congrats !",
		"",
	}, "\n")
	if stdout.String() != wantOutput {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantOutput)
	}

	if len(calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(calls))
	}
	wantClone := []string{
		"clone",
		"--branch", readVersion(t),
		"--depth", "1",
		"--single-branch",
		sampleRepository,
		destination,
	}
	if calls[0].command != "git" || !reflect.DeepEqual(calls[0].args, wantClone) {
		t.Fatalf("clone = %s %#v, want git %#v", calls[0].command, calls[0].args, wantClone)
	}
	if got, want := calls[1].args, []string{"mod", "tidy"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tidy args = %#v, want %#v", got, want)
	}
	if got, want := calls[2].args, []string{"test", "./..."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test args = %#v, want %#v", got, want)
	}
	for _, call := range calls {
		if !call.options.Capture {
			t.Fatalf("%s was not captured", call.command)
		}
	}
}

func TestDoesNotOverwriteDestination(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "my_app"), 0o755); err != nil {
		t.Fatal(err)
	}

	command := Command{
		Version: readVersion(t),
		Dir:     dir,
		Stdout:  &bytes.Buffer{},
		Runner: func(string, []string, commands.Options) error {
			t.Fatal("runner should not be called")
			return nil
		},
	}

	err := command.Execute("github.com/guillermo/my_app")
	if err == nil || !strings.Contains(err.Error(), `destination "my_app" already exists`) {
		t.Fatalf("error = %v", err)
	}
}

func writeFile(t *testing.T, filename, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFileContains(t *testing.T, filename, expected string) {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), expected) {
		t.Fatalf("%s = %q, does not contain %q", filename, data, expected)
	}
}
