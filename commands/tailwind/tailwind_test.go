package tailwindcommand

import (
	"bytes"
	"encoding/json"
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

func TestCommandPreparesInstallsAndBuilds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(dir, "app", "public", "styles.css"), ":root { color-scheme: light dark; }\n")

	var calls []invocation
	command := Command{
		Dir:    filepath.Join(dir, "app", "controllers"),
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
		t.Fatalf("code = %d, want 0", code)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].command != "npm" || !reflect.DeepEqual(calls[0].args, []string{"install"}) {
		t.Fatalf("install call = %s %#v", calls[0].command, calls[0].args)
	}
	if calls[0].options.Dir != dir {
		t.Fatalf("install dir = %q, want %q", calls[0].options.Dir, dir)
	}
	if calls[1].command != "npx" {
		t.Fatalf("tailwind command = %q, want npx", calls[1].command)
	}
	wantArgs := []string{"@tailwindcss/cli", "-i", "app/styles/application.css", "-o", "app/public/styles.css"}
	if !reflect.DeepEqual(calls[1].args, wantArgs) {
		t.Fatalf("tailwind args = %#v, want %#v", calls[1].args, wantArgs)
	}

	input := readFile(t, filepath.Join(dir, "app", "styles", "application.css"))
	if !strings.Contains(input, `@import "tailwindcss";`) {
		t.Fatalf("input = %q, want Tailwind import", input)
	}
	if !strings.Contains(input, "color-scheme") {
		t.Fatalf("input = %q, want seeded existing CSS", input)
	}

	document := readJSONFile(t, filepath.Join(dir, "package.json"))
	if document["private"] != true {
		t.Fatalf("private = %#v, want true", document["private"])
	}
	devDependencies := document["devDependencies"].(map[string]any)
	if devDependencies["tailwindcss"] != "latest" {
		t.Fatalf("tailwindcss dependency = %#v", devDependencies["tailwindcss"])
	}
	if devDependencies["@tailwindcss/cli"] != "latest" {
		t.Fatalf("cli dependency = %#v", devDependencies["@tailwindcss/cli"])
	}
}

func TestCommandUsesExplicitPathsAndWatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(dir, "assets", "tailwind.css"), "@import \"tailwindcss\";\n")

	var calls []invocation
	command := Command{
		Dir:    dir,
		Input:  "assets/tailwind.css",
		Output: "public/tailwind.css",
		Watch:  true,
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
		t.Fatalf("code = %d, want 0", code)
	}
	wantArgs := []string{"@tailwindcss/cli", "-i", "assets/tailwind.css", "-o", "public/tailwind.css", "--watch"}
	if !reflect.DeepEqual(calls[1].args, wantArgs) {
		t.Fatalf("tailwind args = %#v, want %#v", calls[1].args, wantArgs)
	}
}

func TestCommandDefaultsToRootPublicWhenAppPublicIsMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(dir, "public", "styles.css"), "body { margin: 0; }\n")

	var calls []invocation
	command := Command{
		Dir: dir,
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
		t.Fatalf("code = %d, want 0", code)
	}
	wantArgs := []string{"@tailwindcss/cli", "-i", "styles/application.css", "-o", "public/styles.css"}
	if !reflect.DeepEqual(calls[1].args, wantArgs) {
		t.Fatalf("tailwind args = %#v, want %#v", calls[1].args, wantArgs)
	}
	if input := readFile(t, filepath.Join(dir, "styles", "application.css")); !strings.Contains(input, "margin") {
		t.Fatalf("input = %q, want seeded root stylesheet", input)
	}
}

func TestCommandDetectsPnpm(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\n")

	var calls []invocation
	command := Command{
		Dir: dir,
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
		t.Fatalf("code = %d, want 0", code)
	}
	if calls[0].command != "pnpm" {
		t.Fatalf("install command = %q, want pnpm", calls[0].command)
	}
}

func TestCommandRejectsMatchingInputAndOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")

	code, err := (Command{
		Dir:    dir,
		Input:  "styles.css",
		Output: "styles.css",
	}).Execute()
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "must be different") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsurePackageDevDependenciesPreservesExistingVersions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	writeFile(t, path, `{
  "private": true,
  "dependencies": {
    "tailwindcss": "^4.0.0"
  },
  "devDependencies": {
    "@tailwindcss/cli": "^4.0.0"
  }
}`)

	changed, err := ensurePackageDevDependencies(path, requiredPackages)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
}

func TestEnsurePackageDevDependenciesDoesNotAddEmptyDevDependencies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	writeFile(t, path, `{
  "private": true,
  "dependencies": {
    "@tailwindcss/cli": "^4.0.0",
    "tailwindcss": "^4.0.0"
  }
}`)

	changed, err := ensurePackageDevDependencies(path, requiredPackages)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}

	document := readJSONFile(t, path)
	if _, ok := document["devDependencies"]; ok {
		t.Fatalf("devDependencies = %#v, want absent", document["devDependencies"])
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}
