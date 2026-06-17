package jscommand

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golazy/lazy/commands"
)

func TestOpenManifestEditsAndCloses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(dir, "js.toml"), `
[entrypoint.turbo]
module = "@hotwired/turbo"
`)

	editor, err := OpenManifest(filepath.Join(dir, "app", "controllers"))
	if err != nil {
		t.Fatal(err)
	}

	var installDir string
	editor.Runner = func(name string, args []string, options commands.Options) error {
		if name != "npm" {
			t.Fatalf("install command = %q, want npm", name)
		}
		installDir = options.Dir
		fmt.Fprintln(options.Stdout, "install output")
		return nil
	}

	var bundled Manifest
	editor.Bundler = func(manifest Manifest, root, packageDir string) (BuildResult, error) {
		bundled = cloneManifest(manifest)
		return BuildResult{}, nil
	}

	if err := editor.UpdateEntrypoint("turbo", func(entrypoint *Entrypoint) error {
		entrypoint.Imports = []string{"@hotwired/turbo"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := editor.AddEntrypoint(Entrypoint{
		Name:   "stimulus",
		Group:  "ui",
		Module: "@hotwired/stimulus",
	}); err != nil {
		t.Fatal(err)
	}
	if err := editor.Close(); err != nil {
		t.Fatal(err)
	}

	if installDir != dir {
		t.Fatalf("install dir = %q, want %q", installDir, dir)
	}
	if len(bundled.Entrypoints) != 2 {
		t.Fatalf("bundled entrypoints = %d, want 2", len(bundled.Entrypoints))
	}
	if got, want := bundled.Entrypoints[1].Group, "ui"; got != want {
		t.Fatalf("bundled group = %q, want %q", got, want)
	}

	data, err := os.ReadFile(filepath.Join(dir, "js.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `group = "ui"`) {
		t.Fatalf("js.toml = %s, want explicit group", data)
	}
	document := readJSONFile(t, filepath.Join(dir, "package.json"))
	dependencies := document["dependencies"].(map[string]any)
	if dependencies["@hotwired/stimulus"] != "latest" {
		t.Fatalf("stimulus dependency = %#v", dependencies["@hotwired/stimulus"])
	}
}

func TestManifestEditorCloseRollsBackOnPipelineFailure(t *testing.T) {
	dir := t.TempDir()
	originalManifest := `[entrypoint.turbo]
module = "@hotwired/turbo"
`
	originalPackage := `{
  "dependencies": {
    "@hotwired/turbo": "^8.0.0"
  }
}
`
	originalImportmap := `{"imports":{"@hotwired/turbo":"/assets/lazyshaft/turbo-old.js"}}
`
	originalBundle := "console.log('old');\n"

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(dir, "js.toml"), originalManifest)
	writeFile(t, filepath.Join(dir, "package.json"), originalPackage)
	writeFile(t, filepath.Join(dir, "app", "public", "assets", "importmap.json"), originalImportmap)
	writeFile(t, filepath.Join(dir, "app", "public", "assets", "lazyshaft", "turbo-old.js"), originalBundle)

	editor, err := OpenManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	editor.Runner = func(name string, args []string, options commands.Options) error {
		fmt.Fprintln(options.Stdout, "install output")
		return nil
	}
	editor.Bundler = func(Manifest, string, string) (BuildResult, error) {
		return BuildResult{}, fmt.Errorf("bundle failed")
	}

	if err := editor.AddEntrypoint(Entrypoint{
		Name:   "stimulus",
		Module: "@hotwired/stimulus",
	}); err != nil {
		t.Fatal(err)
	}

	err = editor.Close()
	if err == nil {
		t.Fatal("Close succeeded, want failure")
	}
	var closeErr *ManifestCloseError
	if !errors.As(err, &closeErr) {
		t.Fatalf("error = %T, want *ManifestCloseError", err)
	}
	if closeErr.RollbackErr != nil {
		t.Fatalf("rollback error = %v", closeErr.RollbackErr)
	}
	if !strings.Contains(closeErr.Diff, "+[entrypoint.stimulus]") {
		t.Fatalf("diff = %s, want added stimulus entrypoint", closeErr.Diff)
	}
	if !strings.Contains(closeErr.Output, "* Preparing JavaScript dependencies") ||
		!strings.Contains(closeErr.Output, "* Bundling JavaScript libraries") ||
		!strings.Contains(closeErr.Output, "install output") {
		t.Fatalf("output = %s, want complete command output", closeErr.Output)
	}

	assertFileContent(t, filepath.Join(dir, "js.toml"), originalManifest)
	assertFileContent(t, filepath.Join(dir, "package.json"), originalPackage)
	assertFileContent(t, filepath.Join(dir, "app", "public", "assets", "importmap.json"), originalImportmap)
	assertFileContent(t, filepath.Join(dir, "app", "public", "assets", "lazyshaft", "turbo-old.js"), originalBundle)
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}
