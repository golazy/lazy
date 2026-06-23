package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := execute([]string{"--version"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if got, want := stdout.String(), "lazy "+currentVersion()+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestLazyCmdHandoffRunsConfiguredBinary(t *testing.T) {
	restoreVersionHandoffTestHooks(t)
	dir := t.TempDir()
	current := filepath.Join(dir, "current", lazyExecutableName())
	target := filepath.Join(dir, "master", lazyExecutableName())
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("target"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(lazyCmdEnv, target)
	executable = func() (string, error) {
		return current, nil
	}

	var calls []commandCall
	runCommand = func(command string, args []string, options commandOptions) (int, error) {
		calls = append(calls, commandCall{command: command, args: slices.Clone(args), env: slices.Clone(options.Env)})
		return 37, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := execute([]string{"--version"}, nil, &stdout, &stderr); code != 37 {
		t.Fatalf("exit code = %d, want 37", code)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %#v, want 1 call", calls)
	}
	if calls[0].command != target {
		t.Fatalf("command = %q, want %q", calls[0].command, target)
	}
	if got, want := calls[0].args, []string{"--version"}; !slices.Equal(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if got, want := calls[0].env, []string{noVersionCheckEnv + "=true"}; !slices.Equal(got, want) {
		t.Fatalf("env = %#v, want %#v", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Running "+target+" instead") {
		t.Fatalf("stderr = %q, want handoff message", got)
	}
}

func TestLazyCmdHandoffSkipsWhenAlreadyRunningConfiguredBinary(t *testing.T) {
	restoreVersionHandoffTestHooks(t)
	dir := t.TempDir()
	target := filepath.Join(dir, lazyExecutableName())
	if err := os.WriteFile(target, []byte("target"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(lazyCmdEnv, target)
	executable = func() (string, error) {
		return target, nil
	}
	runCommand = func(string, []string, commandOptions) (int, error) {
		t.Fatalf("runCommand should not be called")
		return 1, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := execute([]string{"--version"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := stdout.String(), "lazy "+currentVersion()+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestNewRequiresModuleName(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"new"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lazy new") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestNewRejectsMissingSourceDirArgument(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"new", "--source-dir"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "flag needs an argument") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestNewRejectsMissingVersionArgument(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"new", "--version"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "flag needs an argument") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestNewRejectsSourceDirWithVersion(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"new", "--source-dir", "../sample_app", "--version", "v0.1.10", "example.com/app"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--source-dir and --version cannot be used together") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRoutesRejectsArguments(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"routes", "extra"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lazy routes") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestJSRejectsArguments(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"js", "extra"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "js does not accept arguments") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestTailwindRejectsArguments(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"tailwind", "extra"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lazy tailwind") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestBastardRejectsArguments(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"bastard", "extra"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lazy bastard") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestUpgradeRejectsExtraArguments(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"upgrade", "extra"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lazy upgrade") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestProjectVersionMatchDoesNotHandoff(t *testing.T) {
	t.Chdir(t.TempDir())
	writeGoMod(t, currentVersion())
	restoreVersionHandoffTestHooks(t)

	runCommand = func(string, []string, commandOptions) (int, error) {
		t.Fatalf("runCommand should not be called")
		return 1, nil
	}

	var stderr bytes.Buffer
	handled, code := maybeExecuteProjectVersion([]string{"routes"}, nil, &bytes.Buffer{}, &stderr)
	if handled {
		t.Fatalf("handled = true, code = %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestProjectVersionMismatchRunsCachedLazy(t *testing.T) {
	t.Chdir(t.TempDir())
	writeGoMod(t, "v0.1.7")
	restoreVersionHandoffTestHooks(t)

	cacheDir := t.TempDir()
	userCacheDir = func() (string, error) {
		return cacheDir, nil
	}
	binary := filepath.Join(cacheDir, "golazy", "lazy", "builds", "v0.1.7", lazyExecutableName())
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	var calls []commandCall
	runCommand = func(command string, args []string, options commandOptions) (int, error) {
		calls = append(calls, commandCall{command: command, args: slices.Clone(args), env: slices.Clone(options.Env)})
		return 23, nil
	}

	var stderr bytes.Buffer
	handled, code := maybeExecuteProjectVersion([]string{"routes", "--cmdpath", "cmd/app"}, nil, &bytes.Buffer{}, &stderr)
	if !handled {
		t.Fatalf("handled = false")
	}
	if code != 23 {
		t.Fatalf("code = %d, want 23", code)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %#v, want 1 call", calls)
	}
	if calls[0].command != binary {
		t.Fatalf("command = %q, want %q", calls[0].command, binary)
	}
	if got, want := calls[0].args, []string{"routes", "--cmdpath", "cmd/app"}; !slices.Equal(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if got, want := calls[0].env, []string{noVersionCheckEnv + "=true"}; !slices.Equal(got, want) {
		t.Fatalf("env = %#v, want %#v", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestProjectVersionMismatchInstallsMissingLazy(t *testing.T) {
	t.Chdir(t.TempDir())
	writeGoMod(t, "v0.1.7")
	restoreVersionHandoffTestHooks(t)

	cacheDir := t.TempDir()
	userCacheDir = func() (string, error) {
		return cacheDir, nil
	}
	binary := filepath.Join(cacheDir, "golazy", "lazy", "builds", "v0.1.7", lazyExecutableName())

	var calls []commandCall
	runCommand = func(command string, args []string, options commandOptions) (int, error) {
		calls = append(calls, commandCall{command: command, args: slices.Clone(args), env: slices.Clone(options.Env)})
		return 0, nil
	}

	var stderr bytes.Buffer
	handled, code := maybeExecuteProjectVersion([]string{"js"}, nil, &bytes.Buffer{}, &stderr)
	if !handled {
		t.Fatalf("handled = false")
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %#v, want install and run", calls)
	}
	if calls[0].command != "go" {
		t.Fatalf("install command = %q, want go", calls[0].command)
	}
	if got, want := calls[0].args, []string{"install", "golazy.dev/lazy@v0.1.7"}; !slices.Equal(got, want) {
		t.Fatalf("install args = %#v, want %#v", got, want)
	}
	if got, want := calls[0].env, []string{"GOBIN=" + filepath.Dir(binary)}; !slices.Equal(got, want) {
		t.Fatalf("install env = %#v, want %#v", got, want)
	}
	if calls[1].command != binary {
		t.Fatalf("run command = %q, want %q", calls[1].command, binary)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestProjectVersionMismatchReportsInstallFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	writeGoMod(t, "v0.1.7")
	restoreVersionHandoffTestHooks(t)

	cacheDir := t.TempDir()
	userCacheDir = func() (string, error) {
		return cacheDir, nil
	}
	runCommand = func(command string, args []string, options commandOptions) (int, error) {
		if command != "go" {
			t.Fatalf("unexpected command = %q", command)
		}
		return 1, errors.New("network unavailable")
	}

	var stderr bytes.Buffer
	handled, code := maybeExecuteProjectVersion([]string{"js"}, nil, &bytes.Buffer{}, &stderr)
	if !handled {
		t.Fatalf("handled = false")
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "install golazy.dev/lazy@v0.1.7: network unavailable") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestProjectVersionCheckCanBeSkippedByEnvironment(t *testing.T) {
	t.Chdir(t.TempDir())
	writeGoMod(t, "v0.1.7")
	restoreVersionHandoffTestHooks(t)
	t.Setenv(noVersionCheckEnv, "true")

	runCommand = func(string, []string, commandOptions) (int, error) {
		t.Fatalf("runCommand should not be called")
		return 1, nil
	}

	handled, code := maybeExecuteProjectVersion([]string{"js"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if handled {
		t.Fatalf("handled = true, code = %d", code)
	}
}

func TestProjectVersionCheckCanBeSkippedByFlag(t *testing.T) {
	t.Chdir(t.TempDir())
	writeGoMod(t, "v0.1.7")
	restoreVersionHandoffTestHooks(t)

	runCommand = func(string, []string, commandOptions) (int, error) {
		t.Fatalf("runCommand should not be called")
		return 1, nil
	}

	handled, code := maybeExecuteProjectVersion([]string{skipVersionCheckFlag, "js"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if handled {
		t.Fatalf("handled = true, code = %d", code)
	}
}

func TestRemoveSkipVersionCheckFlag(t *testing.T) {
	args, skipped := removeSkipVersionCheckFlag([]string{skipVersionCheckFlag, "routes", "--cmdpath", "cmd/app", skipVersionCheckFlag})

	if !skipped {
		t.Fatalf("skipped = false")
	}
	if got, want := args, []string{"routes", "--cmdpath", "cmd/app"}; !slices.Equal(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestProjectVersionCheckSkipsBastard(t *testing.T) {
	t.Chdir(t.TempDir())
	writeGoMod(t, "v0.1.7")
	restoreVersionHandoffTestHooks(t)

	runCommand = func(string, []string, commandOptions) (int, error) {
		t.Fatalf("runCommand should not be called")
		return 1, nil
	}

	handled, code := maybeExecuteProjectVersion([]string{"bastard"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if handled {
		t.Fatalf("handled = true, code = %d", code)
	}
}

func TestProjectVersionCheckSkipsUpgrade(t *testing.T) {
	t.Chdir(t.TempDir())
	writeGoMod(t, "v0.1.7")
	restoreVersionHandoffTestHooks(t)

	runCommand = func(string, []string, commandOptions) (int, error) {
		t.Fatalf("runCommand should not be called")
		return 1, nil
	}

	handled, code := maybeExecuteProjectVersion([]string{"upgrade"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if handled {
		t.Fatalf("handled = true, code = %d", code)
	}
}

type commandCall struct {
	command string
	args    []string
	env     []string
}

func restoreVersionHandoffTestHooks(t *testing.T) {
	t.Helper()

	originalUserCacheDir := userCacheDir
	originalExecutable := executable
	originalStatFile := statFile
	originalMkdirAll := mkdirAll
	originalRunCommand := runCommand
	t.Cleanup(func() {
		userCacheDir = originalUserCacheDir
		executable = originalExecutable
		statFile = originalStatFile
		mkdirAll = originalMkdirAll
		runCommand = originalRunCommand
	})
}

func writeGoMod(t *testing.T, version string) {
	t.Helper()

	data := "module example.com/app\n\ngo 1.26.0\n\nrequire golazy.dev " + version + "\n"
	if err := os.WriteFile("go.mod", []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lazyExecutableName() string {
	name := "lazy"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}
