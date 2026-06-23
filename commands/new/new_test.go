package newcommand

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"golazy.dev/lazy/commands"
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
		Version:         readVersion(t),
		Dir:             dir,
		SkipUpdateCheck: true,
		Stdout:          &stdout,
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			if name != "git" || len(args) == 0 {
				return nil
			}

			switch args[0] {
			case "clone":
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
				writeFile(
					t,
					filepath.Join(destination, "init", "app.go"),
					strings.Join([]string{
						"package appinit",
						"",
						"import \"golazy.dev/lazysession\"",
						"",
						"var _ = lazysession.Config{",
						"    Key: \"template-secure-cookie-key\",",
						"}",
						"",
					}, "\n"),
				)
			case "init":
				if err := os.MkdirAll(filepath.Join(options.Dir, ".git"), 0o755); err != nil {
					return err
				}
			}
			return nil
		},
	}

	if err := command.Execute("github.com/guillermo/my_app"); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(dir, "my_app")
	if _, err := os.Stat(filepath.Join(destination, ".git")); err != nil {
		t.Fatalf(".git was not initialized: %v", err)
	}
	assertFileContains(t, filepath.Join(destination, "go.mod"), "module github.com/guillermo/my_app")
	assertFileContains(t, filepath.Join(destination, "main.go"), `"github.com/guillermo/my_app/app"`)
	assertGeneratedSecureCookieKey(t, filepath.Join(destination, "init", "app.go"))

	wantOutput := strings.Join([]string{
		"* Initializing the core app",
		"* Naming the app",
		"* Preparing the mise development environment",
		"* Validating",
		"* Saving the initial Git commit",
		"Next steps:",
		"  cd my_app",
		"  lazy",
		"",
	}, "\n")
	if stdout.String() != wantOutput {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantOutput)
	}

	if len(calls) != 8 {
		t.Fatalf("calls = %d, want 8", len(calls))
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
	if calls[1].command != "mise" || !reflect.DeepEqual(calls[1].args, []string{"trust", "--yes", "mise.toml"}) {
		t.Fatalf("mise trust = %s %#v", calls[1].command, calls[1].args)
	}
	if calls[2].command != "mise" || !reflect.DeepEqual(calls[2].args, []string{"install", "--yes"}) {
		t.Fatalf("mise install = %s %#v", calls[2].command, calls[2].args)
	}
	if calls[3].command != "mise" {
		t.Fatalf("tidy command = %s, want mise", calls[3].command)
	}
	if got, want := calls[3].args, []string{"exec", "--", "go", "mod", "tidy"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tidy args = %#v, want %#v", got, want)
	}
	if calls[4].command != "mise" {
		t.Fatalf("test command = %s, want mise", calls[4].command)
	}
	if got, want := calls[4].args, []string{"exec", "--", "go", "test", "./..."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test args = %#v, want %#v", got, want)
	}
	assertInitialGitCommitCalls(t, calls[5:], destination)
	for _, call := range []invocation{calls[0], calls[3], calls[4]} {
		if !call.options.Capture {
			t.Fatalf("%s was not captured", call.command)
		}
	}
}

func TestClonesSpecificVersion(t *testing.T) {
	dir := t.TempDir()
	var calls []invocation

	command := Command{
		Version:         "v0.1.10",
		CurrentVersion:  readVersion(t),
		Dir:             dir,
		SkipUpdateCheck: true,
		Stdout:          &bytes.Buffer{},
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			if name == "git" && len(args) > 0 && args[0] == "clone" {
				destination := args[len(args)-1]
				writeFile(t, filepath.Join(destination, "go.mod"), "module sample_app\n")
				writeFile(t, filepath.Join(destination, "main.go"), "package main\n")
			}
			return nil
		},
	}

	if err := command.Execute("github.com/guillermo/my_app"); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || calls[0].command != "git" {
		t.Fatalf("calls = %#v, want first git clone", calls)
	}
	wantClone := []string{
		"clone",
		"--branch", "v0.1.10",
		"--depth", "1",
		"--single-branch",
		sampleRepository,
		filepath.Join(dir, "my_app"),
	}
	if !reflect.DeepEqual(calls[0].args, wantClone) {
		t.Fatalf("clone args = %#v, want %#v", calls[0].args, wantClone)
	}
}

func TestNewStopsWhenNewerVersionIsAvailable(t *testing.T) {
	dir := t.TempDir()
	command := Command{
		Version:        "v0.1.10",
		CurrentVersion: "v0.1.10",
		Dir:            dir,
		Stdout:         &bytes.Buffer{},
		LatestVersionFetcher: func(ctx context.Context, url string) (string, error) {
			if url != defaultLatestVersionURL {
				t.Fatalf("url = %q, want %q", url, defaultLatestVersionURL)
			}
			return "v0.1.11\n", nil
		},
		Runner: func(string, []string, commands.Options) error {
			t.Fatal("runner should not be called")
			return nil
		},
	}

	err := command.Execute("github.com/guillermo/my_app")
	if err == nil {
		t.Fatal("err = nil, want newer version error")
	}
	if !strings.Contains(err.Error(), "lazy v0.1.11 is available") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "--skip-update-check") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewContinuesWhenLatestVersionCheckFails(t *testing.T) {
	dir := t.TempDir()
	var calls []invocation
	command := Command{
		Version:        "v0.1.10",
		CurrentVersion: "v0.1.10",
		Dir:            dir,
		Stdout:         &bytes.Buffer{},
		LatestVersionFetcher: func(context.Context, string) (string, error) {
			return "", errors.New("network unavailable")
		},
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			if name == "git" && len(args) > 0 && args[0] == "clone" {
				destination := args[len(args)-1]
				writeFile(t, filepath.Join(destination, "go.mod"), "module sample_app\n")
				writeFile(t, filepath.Join(destination, "main.go"), "package main\n")
			}
			return nil
		},
	}

	if err := command.Execute("github.com/guillermo/my_app"); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || calls[0].command != "git" {
		t.Fatalf("calls = %#v, want first git clone", calls)
	}
}

func TestRejectsInvalidTemplateVersion(t *testing.T) {
	dir := t.TempDir()
	command := Command{
		Version: "not-a-version",
		Dir:     dir,
		Stdout:  &bytes.Buffer{},
		Runner: func(string, []string, commands.Options) error {
			t.Fatal("runner should not be called")
			return nil
		},
	}

	err := command.Execute("github.com/guillermo/my_app")
	if err == nil || !strings.Contains(err.Error(), `version "not-a-version" is not a valid semantic version`) {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveMiseCommandUsesPathWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, executableName("mise")))
	t.Setenv("PATH", dir)

	command, env := resolveMiseCommand()
	if command != "mise" {
		t.Fatalf("command = %q, want mise", command)
	}
	if len(env) != 0 {
		t.Fatalf("env = %#v, want none", env)
	}
}

func TestResolveMiseCommandFallsBackToHomeLocalBin(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	misePath := filepath.Join(home, ".local", "bin", executableName("mise"))
	writeExecutable(t, misePath)
	pathDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", pathDir)

	command, env := resolveMiseCommand()
	if command != misePath {
		t.Fatalf("command = %q, want %q", command, misePath)
	}
	wantPath := filepath.Dir(misePath) + string(os.PathListSeparator) + pathDir
	if got, want := env, []string{"PATH=" + wantPath}; !reflect.DeepEqual(got, want) {
		t.Fatalf("env = %#v, want %#v", got, want)
	}
}

func TestReplaceSecureCookieKeyGeneratesRandomHexKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "init", "app.go")
	writeFile(t, path, strings.Join([]string{
		"package appinit",
		"",
		"import \"golazy.dev/lazysession\"",
		"",
		"var _ = lazysession.Config{",
		"    Key: \"template-secure-cookie-key\",",
		"}",
		"const otherKey = \"template-secure-cookie-key\"",
		"",
	}, "\n"))

	if err := replaceSecureCookieKey(dir); err != nil {
		t.Fatal(err)
	}

	assertGeneratedSecureCookieKey(t, path)
	assertFileContains(t, path, `const otherKey = "template-secure-cookie-key"`)
}

func TestReplaceSecureCookieKeySupportsLegacyConst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "init", "app.go")
	writeFile(t, path, strings.Join([]string{
		"package appinit",
		"",
		"const secureCookieKey = \"template-secure-cookie-key\"",
		"",
	}, "\n"))

	if err := replaceSecureCookieKey(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `secureCookieKey = "template-secure-cookie-key"`) {
		t.Fatalf("%s still contains the template secure cookie key", path)
	}
	if !regexp.MustCompile(`secureCookieKey = "[a-f0-9]{16}"`).Match(data) {
		t.Fatalf("%s does not contain a generated legacy secure cookie key: %s", path, data)
	}
}

func TestCopiesSourceDirectoryRenamesAndValidates(t *testing.T) {
	t.Setenv("GOWORK", "off")

	dir := t.TempDir()
	source := filepath.Join(dir, "sample_app")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(source, "go.mod"), "module sample_app\n")
	writeFile(t, filepath.Join(source, "node_modules", "library", "index.js"), "export {}\n")
	writeFile(
		t,
		filepath.Join(source, "main.go"),
		"package main\nimport \"sample_app/app\"\n",
	)

	var stdout bytes.Buffer
	var calls []invocation

	command := Command{
		Version:   readVersion(t),
		SourceDir: source,
		Dir:       dir,
		Stdout:    &stdout,
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			if name == "git" && len(args) > 0 && args[0] == "init" {
				return os.MkdirAll(filepath.Join(options.Dir, ".git"), 0o755)
			}
			return nil
		},
	}

	if err := command.Execute("github.com/guillermo/my_app"); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(dir, "my_app")
	if _, err := os.Stat(filepath.Join(destination, ".git")); err != nil {
		t.Fatalf(".git was not initialized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("node_modules was copied: %v", err)
	}
	assertFileContains(t, filepath.Join(destination, "go.mod"), "module github.com/guillermo/my_app")
	assertFileContains(t, filepath.Join(destination, "main.go"), `"github.com/guillermo/my_app/app"`)

	if len(calls) != 7 {
		t.Fatalf("calls = %d, want 7", len(calls))
	}
	if calls[0].command != "mise" || !reflect.DeepEqual(calls[0].args, []string{"trust", "--yes", "mise.toml"}) {
		t.Fatalf("mise trust = %s %#v", calls[0].command, calls[0].args)
	}
	if calls[1].command != "mise" || !reflect.DeepEqual(calls[1].args, []string{"install", "--yes"}) {
		t.Fatalf("mise install = %s %#v", calls[1].command, calls[1].args)
	}
	if calls[2].command != "mise" {
		t.Fatalf("tidy command = %s, want mise", calls[2].command)
	}
	if got, want := calls[2].args, []string{"exec", "--", "go", "mod", "tidy"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tidy args = %#v, want %#v", got, want)
	}
	if calls[3].command != "mise" {
		t.Fatalf("test command = %s, want mise", calls[3].command)
	}
	if got, want := calls[3].args, []string{"exec", "--", "go", "test", "./..."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test args = %#v, want %#v", got, want)
	}
	assertInitialGitCommitCalls(t, calls[4:], destination)
}

func TestCopiesSourceDirectoryValidatesWithWorkspaceReplaces(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "golazy"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(
		t,
		filepath.Join(dir, "go.work"),
		"go 1.26.0\n\nreplace golazy.dev v0.1.4 => ./golazy\n",
	)
	t.Setenv("GOWORK", filepath.Join(dir, "go.work"))

	source := filepath.Join(dir, "sample_app")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(
		t,
		filepath.Join(source, "go.mod"),
		"module sample_app\n\ngo 1.26.0\n\nrequire golazy.dev v0.1.4\n",
	)
	writeFile(
		t,
		filepath.Join(source, "main.go"),
		"package main\nimport \"sample_app/app\"\n",
	)

	var calls []invocation
	command := Command{
		Version:   "v0.1.4",
		SourceDir: source,
		Dir:       dir,
		Stdout:    &bytes.Buffer{},
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			if name == "git" && len(args) > 0 {
				switch args[0] {
				case "init":
					return os.MkdirAll(filepath.Join(options.Dir, ".git"), 0o755)
				case "add":
					if _, err := os.Stat(filepath.Join(options.Dir, ".lazy-go.mod")); !os.IsNotExist(err) {
						t.Fatalf(".lazy-go.mod exists before git add: %v", err)
					}
					if _, err := os.Stat(filepath.Join(options.Dir, ".lazy-go.sum")); !os.IsNotExist(err) {
						t.Fatalf(".lazy-go.sum exists before git add: %v", err)
					}
				}
			}
			return nil
		},
	}

	if err := command.Execute("github.com/guillermo/my_app"); err != nil {
		t.Fatal(err)
	}

	if len(calls) != 7 {
		t.Fatalf("calls = %d, want 7", len(calls))
	}
	if calls[0].command != "mise" || !reflect.DeepEqual(calls[0].args, []string{"trust", "--yes", "mise.toml"}) {
		t.Fatalf("mise trust = %s %#v", calls[0].command, calls[0].args)
	}
	if calls[1].command != "mise" || !reflect.DeepEqual(calls[1].args, []string{"install", "--yes"}) {
		t.Fatalf("mise install = %s %#v", calls[1].command, calls[1].args)
	}
	if calls[2].command != "mise" {
		t.Fatalf("tidy command = %s, want mise", calls[2].command)
	}
	if got, want := calls[2].args, []string{"exec", "--", "go", "mod", "tidy", "-modfile=.lazy-go.mod"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tidy args = %#v, want %#v", got, want)
	}
	if calls[3].command != "mise" {
		t.Fatalf("test command = %s, want mise", calls[3].command)
	}
	if got, want := calls[3].args, []string{"exec", "--", "go", "test", "-modfile=.lazy-go.mod", "./..."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test args = %#v, want %#v", got, want)
	}
	for _, call := range []invocation{calls[2], calls[3]} {
		if !contains(call.options.Env, "GOWORK=off") {
			t.Fatalf("%s env = %#v, does not contain GOWORK=off", call.command, call.options.Env)
		}
	}

	destination := filepath.Join(dir, "my_app")
	assertInitialGitCommitCalls(t, calls[4:], destination)
	if _, err := os.Stat(filepath.Join(destination, ".lazy-go.mod")); !os.IsNotExist(err) {
		t.Fatalf(".lazy-go.mod still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, ".lazy-go.sum")); !os.IsNotExist(err) {
		t.Fatalf(".lazy-go.sum still exists: %v", err)
	}
	assertFileContains(t, filepath.Join(destination, "go.mod"), "require golazy.dev v0.1.4")
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

func TestRejectsMissingSourceDirectory(t *testing.T) {
	dir := t.TempDir()

	command := Command{
		Version:   readVersion(t),
		SourceDir: filepath.Join(dir, "missing"),
		Dir:       dir,
		Stdout:    &bytes.Buffer{},
	}

	err := command.Execute("github.com/guillermo/my_app")
	if err == nil || !strings.Contains(err.Error(), "inspect source dir") {
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

func writeExecutable(t *testing.T, filename string) {
	t.Helper()
	writeFile(t, filename, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filename, 0o755); err != nil {
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

func assertGeneratedSecureCookieKey(t *testing.T, filename string) {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `secureCookieKey = "template-secure-cookie-key"`) {
		t.Fatalf("%s still contains the template secure cookie key", filename)
	}
	if strings.Contains(string(data), `Key: "template-secure-cookie-key"`) {
		t.Fatalf("%s still contains the template secure cookie key", filename)
	}
	if !regexp.MustCompile(`Key: "[a-f0-9]{16}"`).Match(data) {
		t.Fatalf("%s does not contain a generated 16-character hex secure cookie key: %s", filename, data)
	}
}

func assertInitialGitCommitCalls(t *testing.T, calls []invocation, destination string) {
	t.Helper()

	want := [][]string{
		{"init"},
		{"add", "."},
		{"commit", "-m", "Initial GoLazy application"},
	}
	if len(calls) != len(want) {
		t.Fatalf("git calls = %d, want %d", len(calls), len(want))
	}
	for index, call := range calls {
		if call.command != "git" {
			t.Fatalf("call %d command = %q, want git", index, call.command)
		}
		if !reflect.DeepEqual(call.args, want[index]) {
			t.Fatalf("call %d args = %#v, want %#v", index, call.args, want[index])
		}
		if call.options.Dir != destination {
			t.Fatalf("call %d dir = %q, want %q", index, call.options.Dir, destination)
		}
		if !call.options.Capture {
			t.Fatalf("call %d was not captured", index)
		}
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
