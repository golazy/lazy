package run

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/appcmd"
)

type invocation struct {
	command string
	args    []string
	options commands.Options
}

func TestUsesFirstCommandUnderCmd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module github.com/golazy/sample_app\n")
	writeFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	writeFile(t, filepath.Join(dir, "app", "public", ".keep"), "")
	writeCommand(t, filepath.Join(dir, "cmd", "sample_app"))
	writeCommand(t, filepath.Join(dir, "cmd", "admin"))

	var calls []invocation
	command := Command{
		Dir:    dir,
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			return nil
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].command != "go" {
		t.Fatalf("tidy command = %s, want go", calls[0].command)
	}
	if got, want := calls[0].args, []string{"mod", "tidy"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tidy args = %#v, want %#v", got, want)
	}
	if !calls[0].options.Capture {
		t.Fatalf("go mod tidy was not captured")
	}
	if got, want := calls[1].args, []string{
		"run",
		"-tags",
		"lazydev",
		"-ldflags",
		appcmd.LazyDevLDFlags(appcmd.LazyDevPaths{
			Views:  filepath.Join(dir, "app", "views"),
			Public: filepath.Join(dir, "app", "public"),
		}),
		"./cmd/admin",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if got, want := calls[1].options.Env, []string(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("env = %#v, want %#v", got, want)
	}
}

func TestUsesExplicitCommandPathAndViewPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/my_app\n")
	writeFile(t, filepath.Join(dir, "views", "layouts", "app.html.tpl"), "layout")
	writeFile(t, filepath.Join(dir, "public_files", ".keep"), "")
	writeCommand(t, filepath.Join(dir, "cmd", "app"))
	writeCommand(t, dir)

	var calls []invocation
	command := Command{
		Dir:        dir,
		CmdPath:    ".",
		ViewPath:   "views",
		PublicPath: "public_files",
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			return nil
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if got, want := calls[0].args, []string{"mod", "tidy"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tidy args = %#v, want %#v", got, want)
	}
	if got, want := calls[1].args, []string{
		"run",
		"-tags",
		"lazydev",
		"-ldflags",
		appcmd.LazyDevLDFlags(appcmd.LazyDevPaths{
			Views:  filepath.Join(dir, "views"),
			Public: filepath.Join(dir, "public_files"),
		}),
		".",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if got, want := calls[1].options.Env, []string(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("env = %#v, want %#v", got, want)
	}
}

func TestSkipsGoModTidyWhenWorkspaceModeIsActive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/my_app\n")
	writeFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	writeFile(t, filepath.Join(dir, "app", "public", ".keep"), "")
	writeCommand(t, filepath.Join(dir, "cmd", "app"))

	var calls []invocation
	command := Command{
		Dir:    dir,
		GoWork: filepath.Join(dir, "go.work"),
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			return nil
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if got, want := calls[0].args, []string{
		"run",
		"-tags",
		"lazydev",
		"-ldflags",
		appcmd.LazyDevLDFlags(appcmd.LazyDevPaths{
			Views:  filepath.Join(dir, "app", "views"),
			Public: filepath.Join(dir, "app", "public"),
		}),
		"./cmd/app",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestExecuteDirectUsesProgressOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/my_app\n")
	writeFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	writeFile(t, filepath.Join(dir, "app", "public", ".keep"), "")
	writeCommand(t, filepath.Join(dir, "cmd", "app"))

	var stdout bytes.Buffer
	command := Command{
		Dir:    dir,
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Runner: func(string, []string, commands.Options) error {
			return nil
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "* Update Go modules ... DONE") {
		t.Fatalf("stdout = %q, missing module progress", output)
	}
	if !strings.Contains(output, "* Run application ... DONE") {
		t.Fatalf("stdout = %q, missing run progress", output)
	}
}

func TestExecuteDirectReturnsApplicationExitCode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/my_app\n")
	writeFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	writeFile(t, filepath.Join(dir, "app", "public", ".keep"), "")
	writeCommand(t, filepath.Join(dir, "cmd", "app"))

	var stderr bytes.Buffer
	command := Command{
		Dir:    dir,
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
		Runner: func(_ string, args []string, _ commands.Options) error {
			if len(args) >= 2 && args[0] == "mod" && args[1] == "tidy" {
				return nil
			}
			return &commands.ExitError{Code: 7, Err: errors.New("exit status 7")}
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
	if !strings.Contains(stderr.String(), "error: run application: exit status 7") {
		t.Fatalf("stderr = %q, missing progress error", stderr.String())
	}
}

func TestErrorsWhenCommandIsMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/my_app\n")

	code, err := (Command{Dir: dir}).Execute()
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "./cmd does not exist") {
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

func writeCommand(t *testing.T, path string) {
	t.Helper()
	writeFile(t, filepath.Join(path, "main.go"), "package main\nimport _ \"golazy.dev/lazyapp\"\nfunc main() {}\n")
}
