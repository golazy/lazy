package upgrade

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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
	if !strings.Contains(stdout.String(), "* Upgrade v0.1.10 -> v0.1.11 ... DONE") {
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

func TestUpgradeTo015MigratesContextInitializerToDependencies(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.14\n")
	writeUpgradeFile(t, filepath.Join(dir, "init", "app.go"), `package appinit

import "golazy.dev/lazyapp"

func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name:    "example",
		Context: Context,
	})
}
`)
	writeUpgradeFile(t, filepath.Join(dir, "init", "context.go"), `package appinit

import (
	"context"
	"fmt"
)

func Context(ctx context.Context) (context.Context, error) {
	ctx = context.WithValue(ctx, "ready", true)
	if false {
		return ctx, fmt.Errorf("not ready")
	}
	return ctx, nil
}
`)

	var stdout bytes.Buffer
	code, err := (Command{
		Dir:          dir,
		Target:       "v0.1.15",
		SkipCommands: true,
		Stdout:       &stdout,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	if _, err := os.Stat(filepath.Join(dir, "init", "context.go")); !os.IsNotExist(err) {
		t.Fatalf("init/context.go still exists: %v", err)
	}
	assertUpgradeFileContains(t, filepath.Join(dir, "init", "app.go"), "Dependencies: Dependencies,")
	assertUpgradeFileContains(t, filepath.Join(dir, "init", "dependencies.go"), `"golazy.dev/lazydeps"`)
	assertUpgradeFileContains(t, filepath.Join(dir, "init", "dependencies.go"), "func Dependencies(deps *lazydeps.Scope) error")
	assertUpgradeFileContains(t, filepath.Join(dir, "init", "dependencies.go"), "ctx := deps.Context()")
	assertUpgradeFileContains(t, filepath.Join(dir, "init", "dependencies.go"), "return fmt.Errorf(\"not ready\")")
	assertUpgradeFileContains(t, filepath.Join(dir, "init", "dependencies.go"), "deps.SetContext(ctx)")
	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.15")
}

func TestUpgradeTo015MigratesSEOInitializerToFunction(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.14\n")
	writeUpgradeFile(t, filepath.Join(dir, "init", "app.go"), `package appinit

import (
	"example.com/app/app"
	"example.com/app/services/site"
	"golazy.dev/lazyapp"
	"golazy.dev/lazyseo"
)

func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name: "example",
		Public: app.Public,
		SEO: []lazyseo.Option{
			lazyseo.SiteName(site.Title),
			lazyseo.Description("Example app"),
			lazyseo.Language("en"),
		},
	})
}
`)

	var stdout bytes.Buffer
	code, err := (Command{
		Dir:          dir,
		Target:       "v0.1.15",
		SkipCommands: true,
		Stdout:       &stdout,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	appPath := filepath.Join(dir, "init", "app.go")
	seoPath := filepath.Join(dir, "init", "seo.go")
	assertUpgradeFileContains(t, appPath, "SEO:    SEO,")
	assertUpgradeFileNotContains(t, appPath, "golazy.dev/lazyseo")
	assertUpgradeFileNotContains(t, appPath, "example.com/app/services/site")
	assertUpgradeFileContains(t, seoPath, `"context"`)
	assertUpgradeFileContains(t, seoPath, `"example.com/app/services/site"`)
	assertUpgradeFileContains(t, seoPath, `"golazy.dev/lazyseo"`)
	assertUpgradeFileContains(t, seoPath, "func SEO(ctx context.Context) []lazyseo.Option")
	assertUpgradeFileContains(t, seoPath, "lazyseo.SiteName(site.Title)")
	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.15")
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
		if call.command != "go" || call.dir != dir {
			t.Fatalf("call = %#v", call)
		}
	}
	wantCycle := [][]string{
		{"mod", "tidy"},
		{"test", "./..."},
		{"vet", "./..."},
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
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.15\n")

	code, err := (Command{
		Dir:   dir,
		Force: "v0.1.15",
	}).Execute()
	if err == nil {
		t.Fatal("err = nil, want no next step error")
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(err.Error(), "v0.1.15 has no later upgrade step") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpgradeRejectsUnimplementedNextVersion(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.13\n")

	code, err := (Command{Dir: dir}).Execute()
	if err == nil {
		t.Fatal("err = nil, want unimplemented upgrade error")
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(err.Error(), "upgrade from v0.1.13 to v0.1.14 is not implemented") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpgradeAlreadyCurrentPromptsToRemoveMiseGoTool(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.15\n")
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
	if !strings.Contains(stdout.String(), "already at v0.1.15") {
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
	if !strings.Contains(stdout.String(), "* Upgrade v0.1.12 -> v0.1.13 ... DONE") {
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

func TestUpgradeConflictCanInstallNewVersionWithDatedBackup(t *testing.T) {
	dir := t.TempDir()
	customMise := "[tools]\ngo = \"1.26.0\"\n# custom task layout\n"
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.10\n")
	writeUpgradeFile(t, filepath.Join(dir, "mise.toml"), customMise)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := (Command{
		Dir:          dir,
		Target:       "v0.1.11",
		SkipCommands: true,
		Stdin:        strings.NewReader("i\n"),
		Stdout:       &stdout,
		Stderr:       &stderr,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	backupPath := filepath.Join(dir, "mise.toml-"+time.Now().Format("2006-01-02"))
	assertUpgradeFileContent(t, filepath.Join(dir, "mise.toml"), v011MiseToml)
	assertUpgradeFileContent(t, backupPath, customMise)
	assertUpgradeFileContains(t, filepath.Join(dir, "go.mod"), "golazy.dev v0.1.11")
	assertUpgradeFileContains(t, filepath.Join(dir, ".mise", "tasks", "dev"), "lazy\n")
	if !strings.Contains(stderr.String(), "install the new version") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestUpgradeFileManifestDeletesMatchingRemovedFile(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "old.txt"), "old template\n")

	var stdout bytes.Buffer
	err := (stepExecutor{
		dir:    dir,
		from:   "vOld",
		to:     "vNew",
		stdout: &stdout,
	}).applyFileManifest(upgradeFileManifest{
		From: "vOld",
		To:   "vNew",
		Files: []upgradeFileOperation{{
			Action:   upgradeFileDelete,
			Path:     "old.txt",
			Previous: upgradeManifestContent("old template\n"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertUpgradeFileMissing(t, filepath.Join(dir, "old.txt"))
	if !strings.Contains(stdout.String(), "removed old.txt") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestUpgradeFileManifestHashesRenderedTemplateContent(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeUpgradeFile(t, filepath.Join(dir, "init", "app.go"), "package appinit\n\nimport \"example.com/app/app\"\n")

	var stdout bytes.Buffer
	err := (stepExecutor{
		dir:        dir,
		modulePath: "example.com/app",
		from:       "vOld",
		to:         "vNew",
		stdout:     &stdout,
	}).applyFileManifest(upgradeFileManifest{
		From: "vOld",
		To:   "vNew",
		Files: []upgradeFileOperation{{
			Action:   upgradeFileUpdate,
			Path:     "init/app.go",
			Previous: upgradeManifestContent("package appinit\n\nimport \"sample_app/app\"\n"),
			Target:   upgradeManifestContent("package appinit\n\nimport \"sample_app/newapp\"\n"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertUpgradeFileContent(t, filepath.Join(dir, "init", "app.go"), "package appinit\n\nimport \"example.com/app/newapp\"\n")
	if !strings.Contains(stdout.String(), "updated init/app.go") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestUpgradeFileManifestCanKeepChangedRemovedFile(t *testing.T) {
	dir := t.TempDir()
	writeUpgradeFile(t, filepath.Join(dir, "old.txt"), "custom local file\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := (stepExecutor{
		dir:    dir,
		from:   "vOld",
		to:     "vNew",
		stdin:  strings.NewReader("k\n"),
		stdout: &stdout,
		stderr: &stderr,
	}).applyFileManifest(upgradeFileManifest{
		From: "vOld",
		To:   "vNew",
		Files: []upgradeFileOperation{{
			Action:   upgradeFileDelete,
			Path:     "old.txt",
			Previous: upgradeManifestContent("old template\n"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertUpgradeFileContent(t, filepath.Join(dir, "old.txt"), "custom local file\n")
	if !strings.Contains(stdout.String(), "kept old.txt; this could create issues") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Keeping it could create issues") {
		t.Fatalf("stderr = %q", stderr.String())
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

func assertUpgradeFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists or stat failed with non-missing error: %v", path, err)
	}
}

func assertUpgradeFileNotContains(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want not to contain %q", path, data, want)
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
