package routes

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golazy.dev/lazy/commands"
)

type invocation struct {
	command string
	args    []string
	options commands.Options
}

func TestCommandRunsApplicationWithPrintRoutesTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/my_app\n")
	writeFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	writeCommand(t, filepath.Join(dir, "cmd", "app"))

	var calls []invocation
	var stdout bytes.Buffer
	command := Command{
		Dir:    dir,
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Runner: func(name string, args []string, options commands.Options) ([]byte, error) {
			calls = append(calls, invocation{command: name, args: args, options: options})
			return []byte(`{"method":"GET","path":"/","name":"root","controller":"home","action":"Index","params":{}}` + "\n"), nil
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if got, want := calls[0].command, "go"; got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
	if got, want := calls[0].args, []string{
		"run",
		"-tags",
		"lazydev,printroutes",
		"./cmd/app",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if got, want := calls[0].options.Env, []string{"GOLAZY_VIEW_PATH=" + filepath.Join(dir, "app", "views")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("env = %#v, want %#v", got, want)
	}
	if !strings.Contains(stdout.String(), "root") || !strings.Contains(stdout.String(), "home#Index") {
		t.Fatalf("stdout = %q, want route table", stdout.String())
	}
}

func TestCommandUsesExplicitCommandAndViewPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/blog\n")
	writeFile(t, filepath.Join(dir, "views", "layouts", "app.html.tpl"), "layout")
	writeCommand(t, filepath.Join(dir, "cmd", "blog"))
	writeCommand(t, filepath.Join(dir, "cmd", "app"))

	var calls []invocation
	command := Command{
		Dir:      dir,
		CmdPath:  "cmd/blog",
		ViewPath: "views",
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		Runner: func(name string, args []string, options commands.Options) ([]byte, error) {
			calls = append(calls, invocation{command: name, args: args, options: options})
			return []byte{}, nil
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := calls[0].args, []string{
		"run",
		"-tags",
		"lazydev,printroutes",
		"./cmd/blog",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if got, want := calls[0].options.Env, []string{"GOLAZY_VIEW_PATH=" + filepath.Join(dir, "views")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("env = %#v, want %#v", got, want)
	}
}

func TestParseRoutesRejectsInvalidJSONL(t *testing.T) {
	_, err := parseRoutes([]byte("not json\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("error = %v, want line parse error", err)
	}
}

func TestWriteTableSortsParams(t *testing.T) {
	var out bytes.Buffer
	err := writeTable(&out, []Route{
		{
			Method:     "GET",
			Path:       "/posts/{post_id}/comments/{comment_id}",
			Name:       "comment",
			Controller: "comments",
			Action:     "Show",
			Params:     map[string]bool{"post_id": true, "comment_id": true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "comment_id,post_id") {
		t.Fatalf("stdout = %q, want sorted params", out.String())
	}
}

func TestWriteTablePrefixesNamespacedControllerTarget(t *testing.T) {
	var out bytes.Buffer
	err := writeTable(&out, []Route{
		{
			Method:     "GET",
			Path:       "/admin/posts",
			Name:       "admin_posts",
			Namespace:  "admin",
			Controller: "posts",
			Action:     "Index",
			Params:     map[string]bool{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "admin/posts#Index") {
		t.Fatalf("stdout = %q, want namespaced controller target", out.String())
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
