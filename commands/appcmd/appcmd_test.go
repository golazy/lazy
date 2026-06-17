package appcmd

import (
	"os"
	"path/filepath"
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
