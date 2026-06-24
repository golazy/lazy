package appcmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindUsesFirstCommandThatDependsOnLazyApp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/app\n")
	writeFile(t, filepath.Join(dir, "cmd", "admin", "main.go"), "package main\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "cmd", "web", "main.go"), `package main

import _ "example.com/team/app/init"

func main() {}
`)
	writeFile(t, filepath.Join(dir, "init", "app.go"), `package appinit

import _ "golazy.dev/lazyapp"
`)

	commandPath, err := Find(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commandPath, filepath.Join("cmd", "web"); got != want {
		t.Fatalf("Find() = %q, want %q", got, want)
	}
}

func TestFindUsesExplicitRootCommandPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/team/app\n")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import _ "golazy.dev/lazyapp"

func main() {}
`)

	commandPath, err := Find(dir, ".")
	if err != nil {
		t.Fatal(err)
	}
	if commandPath != "." {
		t.Fatalf("Find() = %q, want .", commandPath)
	}
}

func writeFile(t *testing.T, filename string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGoBuildArgs(t *testing.T) {
	got := GoBuildArgs("lazydev", "cmd/app", "/tmp/app")
	want := []string{
		"build",
		"-tags",
		"lazydev",
		"-o",
		"/tmp/app",
		"./cmd/app",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GoBuildArgs() = %#v, want %#v", got, want)
	}
}

func TestGoRunArgsIncludesBuildFlags(t *testing.T) {
	got := GoRunArgs("lazydev", "cmd/app", "-ldflags", "-X example.Value=dev")
	want := []string{
		"run",
		"-tags",
		"lazydev",
		"-ldflags",
		"-X example.Value=dev",
		"./cmd/app",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GoRunArgs() = %#v, want %#v", got, want)
	}
}

func TestGoBuildArgsOmitsEmptyTags(t *testing.T) {
	got := GoBuildArgs("", "cmd/app", "/tmp/app")
	want := []string{
		"build",
		"-o",
		"/tmp/app",
		"./cmd/app",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GoBuildArgs() = %#v, want %#v", got, want)
	}
}

func TestResolveViewPathUsesDefaultAppViews(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")

	got, err := ResolveViewPath(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "app", "views"); got != want {
		t.Fatalf("ResolveViewPath() = %q, want %q", got, want)
	}
}

func TestLazyDevBuildFlagsResolveViewAndPublicPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	writeFile(t, filepath.Join(dir, "app", "public", ".keep"), "")

	got, err := LazyDevBuildFlags(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-ldflags",
		LazyDevLDFlags(LazyDevPaths{
			Views:  filepath.Join(dir, "app", "views"),
			Public: filepath.Join(dir, "app", "public"),
		}),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LazyDevBuildFlags() = %#v, want %#v", got, want)
	}
}

func TestLazyDevLDFlagsDoNotQuotePathValues(t *testing.T) {
	got := LazyDevLDFlags(LazyDevPaths{
		Views:  "/workspace/sample_app/app/views",
		Public: "/workspace/sample_app/app/public",
	})
	want := "-X golazy.dev/lazyapp.ViewsPath=/workspace/sample_app/app/views -X golazy.dev/lazyapp.PublicPath=/workspace/sample_app/app/public"
	if got != want {
		t.Fatalf("LazyDevLDFlags() = %q, want %q", got, want)
	}
}

func TestLazyDevLDFlagsQuoteWholeAssignmentWhenNeeded(t *testing.T) {
	got := LazyDevLDFlags(LazyDevPaths{
		Views:  "/workspace/sample app/app/views",
		Public: "/workspace/sample app/app/public",
	})
	want := `-X "golazy.dev/lazyapp.ViewsPath=/workspace/sample app/app/views" -X "golazy.dev/lazyapp.PublicPath=/workspace/sample app/app/public"`
	if got != want {
		t.Fatalf("LazyDevLDFlags() = %q, want %q", got, want)
	}
}

func TestResolveViewPathRejectsMissingLayout(t *testing.T) {
	_, err := ResolveViewPath(t.TempDir(), "views")
	if err == nil {
		t.Fatal("ResolveViewPath() error is nil")
	}
}
