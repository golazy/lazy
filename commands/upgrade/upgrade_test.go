package upgrade

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golazy.dev/lazy/commands"
)

type upgradeInvocation struct {
	command string
	args    []string
	dir     string
}

func TestUpgradeTo011AddsMiseTaskFiles(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.10\n")
	writeUpgradeFile(t, filepath.Join(dir, "mise.toml"), v010MiseToml)

	var stdout bytes.Buffer
	code, err := (Command{
		Dir:          dir,
		Target:       "v0.1.11",
		SkipCommands: true,
		Stdout:       &stdout,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.11")
	assertUpgradeFileContent(t, filepath.Join(dir, "mise.toml"), v011MiseToml)
	assertUpgradeFileContent(t, filepath.Join(dir, ".mise", "tasks", "dev"), v011DevTask)
	assertUpgradeFileContent(t, filepath.Join(dir, ".mise", "tasks", "test"), v011TestTask)
	assertExecutable(t, filepath.Join(dir, ".mise", "tasks", "dev"))
	assertExecutable(t, filepath.Join(dir, ".mise", "tasks", "test"))
	if !strings.Contains(stdout.String(), "added .mise/tasks/dev") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestUpgradeTo012MovesServicesAndRewritesImports(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.11\n")
	writeUpgradeFile(t, filepath.Join(dir, "app", "services", "posts", "posts.go"), "package posts\n")
	writeUpgradeFile(t, filepath.Join(dir, "app", "services", "timeservice", "timeservice.go"), "package timeservice\n")
	writeUpgradeFile(t, filepath.Join(dir, "app", "controllers", "posts", "posts_controller.go"), `package posts

import (
	postservice "example.com/app/app/services/posts"
	"example.com/app/lib/markdown"
)

var _ = postservice.Service{}
`)
	writeUpgradeFile(t, filepath.Join(dir, "init", "context.go"), `package appinit

import "example.com/app/app/services/timeservice"

var _ = timeservice.Service{}
`)

	var stdout bytes.Buffer
	code, err := (Command{
		Dir:          dir,
		Target:       "v0.1.12",
		SkipCommands: true,
		Stdout:       &stdout,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	if _, err := os.Stat(filepath.Join(dir, "app", "services")); !os.IsNotExist(err) {
		t.Fatalf("app/services still exists: %v", err)
	}
	assertUpgradeFileContent(t, filepath.Join(dir, "services", "posts", "posts.go"), "package posts\n")
	assertUpgradeFileContent(t, filepath.Join(dir, "services", "timeservice", "timeservice.go"), "package timeservice\n")
	assertUpgradeFileContains(t, filepath.Join(dir, "app", "controllers", "posts", "posts_controller.go"), `"example.com/app/services/posts"`)
	assertUpgradeFileContains(t, filepath.Join(dir, "app", "controllers", "posts", "posts_controller.go"), "var _ = postservice.Service{}")
	assertUpgradeFileContains(t, filepath.Join(dir, "init", "context.go"), `"example.com/app/services/timeservice"`)
	assertUpgradeFileContains(t, filepath.Join(dir, "init", "context.go"), "var _ = timeservice.Service{}")
	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.12")
}

func TestUpgradeTargetRunsEachStepInOrder(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.10\n")
	writeUpgradeFile(t, filepath.Join(dir, "mise.toml"), v010MiseToml)
	writeUpgradeFile(t, filepath.Join(dir, "app", "services", "posts", "posts.go"), "package posts\n")
	writeUpgradeFile(t, filepath.Join(dir, "app", "controllers", "posts", "posts_controller.go"), `package posts

import postservice "example.com/app/app/services/posts"

var _ = postservice.Service{}
`)

	var calls []upgradeInvocation
	code, err := (Command{
		Dir:    dir,
		Target: "v0.1.12",
		Runner: func(command string, args []string, options commands.Options) error {
			calls = append(calls, upgradeInvocation{
				command: command,
				args:    slices.Clone(args),
				dir:     options.Dir,
			})
			return nil
		},
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(calls) != 6 {
		t.Fatalf("calls = %#v, want six follow-up commands", calls)
	}
	for _, call := range calls {
		if call.command != "mise" || call.dir != dir {
			t.Fatalf("call = %#v", call)
		}
	}
	wantCycle := [][]string{
		{"exec", "--", "go", "mod", "tidy"},
		{"exec", "--", "go", "test", "./..."},
		{"exec", "--", "go", "vet", "./..."},
	}
	for index, call := range calls {
		if !slices.Equal(call.args, wantCycle[index%len(wantCycle)]) {
			t.Fatalf("call %d args = %#v, want %#v", index, call.args, wantCycle[index%len(wantCycle)])
		}
	}
	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.12")
	assertUpgradeFileContains(t, filepath.Join(dir, "app", "controllers", "posts", "posts_controller.go"), `"example.com/app/services/posts"`)
}

func TestUpgradeForceRunsSpecificStepRegardlessCurrentVersion(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.11\n")
	writeUpgradeFile(t, filepath.Join(dir, "mise.toml"), v010MiseToml)

	code, err := (Command{
		Dir:          dir,
		Force:        "v0.1.10",
		SkipCommands: true,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	assertUpgradeFileContent(t, filepath.Join(dir, "mise.toml"), v011MiseToml)
	assertUpgradeFileContent(t, filepath.Join(dir, ".mise", "tasks", "dev"), v011DevTask)
	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.11")
}

func TestUpgradeForceRejectsTarget(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.11\n")

	code, err := (Command{
		Dir:    dir,
		Force:  "v0.1.10",
		Target: "v0.1.12",
	}).Execute()
	if err == nil {
		t.Fatal("err = nil, want mutual exclusion error")
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(err.Error(), "--force and --target cannot be used together") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpgradeForceRejectsLatestVersion(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.13\n")

	code, err := (Command{
		Dir:   dir,
		Force: "v0.1.13",
	}).Execute()
	if err == nil {
		t.Fatal("err = nil, want no next step error")
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(err.Error(), "v0.1.13 has no later upgrade step") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpgradeAlreadyCurrentPromptsToRemoveMiseGoTool(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.13\n")
	writeUpgradeFile(t, filepath.Join(dir, "mise.toml"), "[tools]\ngo = \"1.26.0\"\nnode = \"24\"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := (Command{
		Dir:    dir,
		Stdin:  strings.NewReader("y\n"),
		Stdout: &stdout,
		Stderr: &stderr,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	assertUpgradeFileContent(t, filepath.Join(dir, "mise.toml"), "[tools]\nnode = \"24\"\n")
	if !strings.Contains(stderr.String(), "Go already bundles multi-version support") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "already at v0.1.13") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestUpgradeTo013UpdatesGoModAndPromptsToRemoveMiseGoTool(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.12\n")
	writeUpgradeFile(t, filepath.Join(dir, "mise.toml"), "[tools]\ngo = \"1.26.0\"\nnode = \"24\"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := (Command{
		Dir:          dir,
		Stdin:        strings.NewReader("y\n"),
		Stdout:       &stdout,
		Stderr:       &stderr,
		SkipCommands: true,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.13")
	assertUpgradeFileContent(t, filepath.Join(dir, "mise.toml"), "[tools]\nnode = \"24\"\n")
	if !strings.Contains(stdout.String(), "* Upgrading v0.1.12 -> v0.1.13") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Go already bundles multi-version support") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestUpgradeDryRunTargetDoesNotRequireIntermediateWrites(t *testing.T) {
	dir := t.TempDir()
	originalGoMod := "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.10\n"
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), originalGoMod)
	writeUpgradeFile(t, filepath.Join(dir, "mise.toml"), v010MiseToml)
	writeUpgradeFile(t, filepath.Join(dir, "app", "services", "posts", "posts.go"), "package posts\n")
	writeUpgradeFile(t, filepath.Join(dir, "app", "controllers", "posts", "posts_controller.go"), `package posts

import postservice "example.com/app/app/services/posts"

var _ = postservice.Service{}
`)

	var stdout bytes.Buffer
	code, err := (Command{
		Dir:    dir,
		Target: "v0.1.12",
		DryRun: true,
		Stdout: &stdout,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	assertUpgradeFileContent(t, filepath.Join(dir, "go.mod"), originalGoMod)
	if _, err := os.Stat(filepath.Join(dir, ".mise", "tasks", "dev")); !os.IsNotExist(err) {
		t.Fatalf(".mise/tasks/dev exists after dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "services")); !os.IsNotExist(err) {
		t.Fatalf("services exists after dry-run: %v", err)
	}
	if !strings.Contains(stdout.String(), "would update go.mod to golazy.dev v0.1.12") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestUpgradeConflictWritesProposedFileAndLeavesCurrentFile(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.10\n")
	writeUpgradeFile(t, filepath.Join(dir, "mise.toml"), "[tools]\ngo = \"1.26.0\"\n# custom task layout\n")

	var stderr bytes.Buffer
	code, err := (Command{
		Dir:          dir,
		Target:       "v0.1.11",
		SkipCommands: true,
		Stderr:       &stderr,
	}).Execute()
	if err == nil {
		t.Fatal("err = nil, want conflict")
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	assertUpgradeFileContent(t, filepath.Join(dir, "mise.toml"), "[tools]\ngo = \"1.26.0\"\n# custom task layout\n")
	assertUpgradeFileContent(t, filepath.Join(dir, ".golazy", "upgrade", "conflicts", "v0.1.11", "mise.toml"), v011MiseToml)
	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.10")
	if !strings.Contains(stderr.String(), "--- mise.toml") || !strings.Contains(stderr.String(), "+++ proposed/mise.toml") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(err.Error(), "upgrade conflict in mise.toml") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpgradeStartsAtFirstAwareVersionForOlderApps(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.5\n")
	writeUpgradeFile(t, filepath.Join(dir, "app", "services", "posts", "posts.go"), "package posts\n")
	writeUpgradeFile(t, filepath.Join(dir, "app", "controllers", "posts", "posts_controller.go"), `package posts

import postservice "example.com/app/app/services/posts"

var _ = postservice.Service{}
`)

	var stdout bytes.Buffer
	code, err := (Command{
		Dir:          dir,
		Target:       "v0.1.12",
		SkipCommands: true,
		Stdout:       &stdout,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.12")
	assertUpgradeFileContent(t, filepath.Join(dir, "mise.toml"), v011MiseToml)
	assertUpgradeFileContent(t, filepath.Join(dir, ".mise", "tasks", "dev"), v011DevTask)
	assertUpgradeFileContent(t, filepath.Join(dir, "services", "posts", "posts.go"), "package posts\n")
	assertUpgradeFileContains(t, filepath.Join(dir, "app", "controllers", "posts", "posts_controller.go"), `"example.com/app/services/posts"`)
	if !strings.Contains(stdout.String(), "starting automated migrations at v0.1.10 -> v0.1.11") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func writeUpgradeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertUpgradeFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}

func assertUpgradeFileContains(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want substring %q", path, data, want)
	}
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("%s mode = %v, want executable", path, info.Mode())
	}
}
