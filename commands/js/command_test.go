package jscommand

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"golazy.dev/lazy/commands"
)

type invocation struct {
	command string
	args    []string
	options commands.Options
}

func TestCommandPreparesInstallsAndBundles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(dir, "js.toml"), `
[entrypoint.turbo]
module = "@hotwired/turbo"
`)

	var calls []invocation
	var bundledManifest Manifest
	var bundledRoot string
	var bundledPackageDir string
	command := Command{
		Dir:    filepath.Join(dir, "app", "controllers"),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			return nil
		},
		Bundler: func(manifest Manifest, root, packageDir string) (BuildResult, error) {
			bundledManifest = manifest
			bundledRoot = root
			bundledPackageDir = packageDir
			return BuildResult{}, nil
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].command != "npm" || !reflect.DeepEqual(calls[0].args, []string{"install"}) {
		t.Fatalf("install call = %s %#v", calls[0].command, calls[0].args)
	}
	if calls[0].options.Dir != dir {
		t.Fatalf("install dir = %q, want %q", calls[0].options.Dir, dir)
	}
	if bundledRoot != dir {
		t.Fatalf("bundled root = %q, want %q", bundledRoot, dir)
	}
	if bundledPackageDir != dir {
		t.Fatalf("bundled package dir = %q, want %q", bundledPackageDir, dir)
	}
	if len(bundledManifest.Entrypoints) != 1 {
		t.Fatalf("bundled entrypoints = %d", len(bundledManifest.Entrypoints))
	}

	document := readJSONFile(t, filepath.Join(dir, "package.json"))
	dependencies := document["dependencies"].(map[string]any)
	if dependencies["@hotwired/turbo"] != "latest" {
		t.Fatalf("turbo dependency = %#v", dependencies["@hotwired/turbo"])
	}
}

func TestCommandUsesMisePnpm(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(dir, "js.toml"), `
[entrypoint.turbo]
module = "@hotwired/turbo"
`)

	var calls []invocation
	command := Command{
		Dir:    dir,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Mise:   fakeMiseTools("pnpm"),
		Runner: func(name string, args []string, options commands.Options) error {
			calls = append(calls, invocation{command: name, args: args, options: options})
			return nil
		},
		Bundler: func(Manifest, string, string) (BuildResult, error) {
			return BuildResult{}, nil
		},
	}

	code, err := command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if calls[0].command != "mise" {
		t.Fatalf("command = %q, want mise", calls[0].command)
	}
	if got, want := calls[0].args, []string{"exec", "--", "pnpm", "install"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func fakeMiseTools(names ...string) commands.OutputRunner {
	return func(string, []string, commands.Options) ([]byte, error) {
		var out bytes.Buffer
		out.WriteByte('{')
		for index, name := range names {
			if index > 0 {
				out.WriteByte(',')
			}
			fmt.Fprintf(&out, "%q:[{\"installed\":true,\"active\":true}]", name)
		}
		out.WriteByte('}')
		return out.Bytes(), nil
	}
}

func TestFindAppRootErrorsWhenGoModIsMissing(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	base := filepath.Join(filepath.Dir(filename), "..", "..", "..", ".tmp", "js-command-tests")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(base)
	})
	dir, err := os.MkdirTemp(base, "missing-go-mod-")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := findAppRoot(filepath.Join(dir, "nested")); err == nil {
		t.Fatal("findAppRoot succeeded without go.mod")
	}
}
