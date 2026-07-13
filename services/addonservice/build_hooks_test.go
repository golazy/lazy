package addonservice

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golazy.dev/lazy/services/execservice"
	"golazy.dev/lazycode"
)

func TestBuildHookRunnerUsesDependencyAndIDOrder(t *testing.T) {
	root := t.TempDir()
	writeBuildHookState(t, root, []InstalledAddon{
		buildHookAddon("zeta", nil, "before-build=zeta:two", "before-build=zeta:one", "after-build=zeta:after"),
		buildHookAddon("dependent", []string{"zeta"}, "before-build=dependent:run"),
		buildHookAddon("alpha", nil, "before-build=alpha:run"),
	})

	var calls []string
	var output bytes.Buffer
	runner := BuildHookRunner{
		Root:   root,
		Stdout: &output,
		Stderr: &output,
		Runner: func(command string, args []string, _ execservice.Options) error {
			calls = append(calls, command+" "+strings.Join(args, " "))
			return nil
		},
	}
	if err := runner.Run(BeforeBuild); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"mise run alpha:run",
		"mise run zeta:one",
		"mise run zeta:two",
		"mise run dependent:run",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
	if strings.Contains(output.String(), "zeta:after") {
		t.Fatalf("before-build output contains after hook:\n%s", output.String())
	}
}

func TestBuildHookRunnerStopsAndIdentifiesFailure(t *testing.T) {
	root := t.TempDir()
	writeBuildHookState(t, root, []InstalledAddon{
		buildHookAddon("alpha", nil, "before-build=alpha:first", "before-build=alpha:second"),
	})
	wantErr := errors.New("task failed")
	var calls int
	var output bytes.Buffer
	err := (BuildHookRunner{
		Root:   root,
		Stdout: &output,
		Stderr: &output,
		Runner: func(_ string, _ []string, options execservice.Options) error {
			calls++
			if calls == 1 {
				_, _ = options.Stderr.Write([]byte("hook output\n"))
				return wantErr
			}
			return nil
		},
	}).Run(BeforeBuild)
	if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), `add-on "alpha" before-build hook "alpha:first"`) {
		t.Fatalf("err = %v, want contextual task failure", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want stop after first failure", calls)
	}
	if !strings.Contains(output.String(), "hook output") {
		t.Fatalf("output = %q, want task failure output", output.String())
	}
}

func TestBuildHookRunnerDoesNothingWithoutState(t *testing.T) {
	called := false
	err := (BuildHookRunner{
		Root: t.TempDir(),
		Runner: func(string, []string, execservice.Options) error {
			called = true
			return nil
		},
	}).Run(BeforeBuild)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("runner called without addons.toml")
	}
}

func TestBuildHookPlanKeepsOneLockSnapshotAcrossPhases(t *testing.T) {
	root := t.TempDir()
	writeBuildHookState(t, root, []InstalledAddon{
		buildHookAddon("assets", nil, "before-build=assets:prepare", "after-build=assets:finish"),
	})
	plan, err := LoadBuildHookPlan(root)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	runner := BuildHookRunner{
		Root: root,
		Runner: func(_ string, args []string, _ execservice.Options) error {
			calls = append(calls, args[1])
			if args[1] == "assets:prepare" {
				writeBuildHookState(t, root, []InstalledAddon{
					buildHookAddon("replacement", nil, "after-build=replacement:finish"),
				})
			}
			return nil
		},
	}
	if err := plan.Run(BeforeBuild, runner); err != nil {
		t.Fatal(err)
	}
	if err := plan.Run(AfterBuild, runner); err != nil {
		t.Fatal(err)
	}
	if want := []string{"assets:prepare", "assets:finish"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want original snapshot %#v", calls, want)
	}
}

func TestBuildHookPlanPropagatesContextToTaskRunner(t *testing.T) {
	root := t.TempDir()
	writeBuildHookState(t, root, []InstalledAddon{
		buildHookAddon("assets", nil, "before-build=assets:prepare"),
	})
	plan, err := LoadBuildHookPlan(root)
	if err != nil {
		t.Fatal(err)
	}
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("build"), "hook")
	if err := plan.RunContext(ctx, BeforeBuild, BuildHookRunner{
		Root: root,
		Runner: func(_ string, _ []string, options execservice.Options) error {
			if options.Context != ctx {
				t.Fatalf("task context = %v, want build context", options.Context)
			}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestBuildHookPlanRejectsChangedTaskArtifact(t *testing.T) {
	root := t.TempDir()
	writeBuildHookState(t, root, []InstalledAddon{
		buildHookAddon("assets", nil, "before-build=assets:prepare"),
	})
	filename := filepath.Join(root, ".mise", "tasks", "assets", "prepare")
	if err := os.WriteFile(filename, []byte("changed\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBuildHookPlan(root); err == nil || !strings.Contains(err.Error(), "local changes") {
		t.Fatalf("changed task error = %v", err)
	}
}

func TestPlannerRejectsHookThatClaimsExistingApplicationTask(t *testing.T) {
	root := t.TempDir()
	writeHookTestFile(t, root, "go.mod", "module example.com/app\n\nrequire golazy.dev v0.1.21\n", 0o644)
	writeHookTestFile(t, root, "init/app.go", `package init

import "golazy.dev/lazyapp"

func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{Name: "app"})
}
`, 0o644)
	task := "#!/bin/sh\nexit 0\n"
	writeHookTestFile(t, root, "tasks/build", task, 0o755)
	writeHookTestFile(t, root, ".mise/tasks/assets/build", task, 0o755)
	writeHookTestFile(t, root, "lazyaddon.toml", `manifest = 1

[package]
id = "example.com/assets"
version = "v1.0.0"

[[addons]]
id = "assets"
go_import = "example.com/assets/addon"
files = ["tasks/build=>.mise/tasks/assets/build"]
tasks = ["assets:build"]
build_hooks = ["before-build=assets:build"]
`, 0o644)

	_, err := (Manager{Dir: root, ManifestPaths: []string{"lazyaddon.toml"}}).Add("assets")
	if err == nil || !strings.Contains(err.Error(), "pre-existing application tasks cannot be hooked") {
		t.Fatalf("existing application task error = %v", err)
	}
}

func TestValidateBuildContributionsRequiresTypedDeclaredTask(t *testing.T) {
	for name, build := range map[string]BuildContributions{
		"opaque hook":     {Hooks: []string{"before-build"}, Tasks: []string{"assets:build"}},
		"unknown phase":   {Hooks: []string{"during-build=assets:build"}, Tasks: []string{"assets:build"}},
		"missing task":    {Hooks: []string{"before-build=assets:build"}},
		"invalid task":    {Hooks: []string{"before-build=assets build"}, Tasks: []string{"assets build"}},
		"hidden task":     {Hooks: []string{"before-build=_private"}, Tasks: []string{"_private"}},
		"undeclared task": {Hooks: []string{"after-build=assets:build"}, Tasks: []string{"assets:other"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateBuildContributions("assets", build); err == nil {
				t.Fatal("validation error = nil")
			}
		})
	}
	if err := validateBuildContributions("assets", BuildContributions{
		Hooks: []string{"before-build=assets:prepare", "after-build=assets:finish"},
		Tasks: []string{"assets:prepare", "assets:finish"},
	}); err != nil {
		t.Fatalf("valid contributions: %v", err)
	}
}

func TestDependencyOrderRejectsMissingAndCycle(t *testing.T) {
	if _, err := dependencyOrder([]InstalledAddon{buildHookAddon("alpha", []string{"missing"})}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing dependency err = %v", err)
	}
	if _, err := dependencyOrder([]InstalledAddon{
		buildHookAddon("alpha", []string{"beta"}),
		buildHookAddon("beta", []string{"alpha"}),
	}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle err = %v", err)
	}
	if _, err := dependencyOrder([]InstalledAddon{
		buildHookAddon("alpha", []string{"beta@v2.0.0"}),
		buildHookAddon("beta", nil),
	}); err == nil || !strings.Contains(err.Error(), "lock contains v1.0.0") {
		t.Fatalf("version mismatch err = %v", err)
	}
}

func buildHookAddon(id string, requires []string, hooks ...string) InstalledAddon {
	for index, requirement := range requires {
		if !strings.Contains(requirement, "@") {
			requires[index] = requirement + "@v1.0.0"
		}
	}
	tasks := make([]string, 0, len(hooks))
	files := make([]string, 0, len(hooks))
	for _, value := range hooks {
		_, task, _ := strings.Cut(value, "=")
		if task != "" {
			tasks = append(tasks, task)
			target := ".mise/tasks/" + strings.ReplaceAll(task, ":", "/")
			files = append(files, "artifacts/"+strings.ReplaceAll(task, ":", "-")+"=>"+target)
		}
	}
	return InstalledAddon{ResolvedAddon: ResolvedAddon{
		ID:              id,
		Version:         "v1.0.0",
		Requires:        requires,
		Package:         "example.com/" + id,
		PackageVersion:  "v1.0.0",
		ManifestDigest:  lazycode.Hash([]byte("manifest:" + id)),
		Source:          "test",
		GoImport:        "example.com/" + id + "/addon",
		GoModule:        "example.com/" + id,
		GoModuleVersion: "v1.0.0",
		Files:           files,
		Build: BuildContributions{
			Hooks: hooks,
			Tasks: tasks,
		},
	}}
}

func writeBuildHookState(t *testing.T, root string, addons []InstalledAddon) {
	t.Helper()
	managedByPath := map[string]ManagedFile{}
	for _, addon := range addons {
		for _, declaration := range addon.Files {
			_, target, mapped := strings.Cut(declaration, "=>")
			if !mapped {
				continue
			}
			target = strings.TrimSpace(target)
			contents := []byte("#!/bin/sh\n# " + target + "\n")
			writeHookTestFile(t, root, target, string(contents), 0o755)
			managed := managedByPath[target]
			managed.Path = target
			managed.Kind = "addon-file"
			managed.Owners = append(managed.Owners, addon.ID)
			managed.Baseline = lazycode.Hash(contents)
			managed.Mode = 0o755
			managedByPath[target] = managed
		}
	}
	managed := make([]ManagedFile, 0, len(managedByPath))
	for _, file := range managedByPath {
		managed = append(managed, file)
	}
	data, err := MarshalState(State{Manifest: 1, Addons: addons, Managed: managed})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, StateFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeHookTestFile(t *testing.T, root, name, contents string, mode os.FileMode) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}
