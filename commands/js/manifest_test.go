package jscommand

import (
	"reflect"
	"testing"
)

func TestParseManifestUsesDefaultsAndEntrypointBlocks(t *testing.T) {
	manifest, err := ParseManifest([]byte(`
[entrypoint.turbo]
module = "@hotwired/turbo"

[entrypoint.stimulus]
module = "@hotwired/stimulus"
`))
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Package != "package.json" {
		t.Fatalf("Package = %q, want package.json", manifest.Package)
	}
	if manifest.Output.Dir != "app/public/assets/lazyshaft" {
		t.Fatalf("Output.Dir = %q", manifest.Output.Dir)
	}
	if manifest.Output.Importmap != "app/public/assets/importmap.json" {
		t.Fatalf("Output.Importmap = %q", manifest.Output.Importmap)
	}
	if !manifest.Bundle.Shared {
		t.Fatal("Bundle.Shared = false, want true")
	}
	if !manifest.Bundle.Minify {
		t.Fatal("Bundle.Minify = false, want true")
	}

	if len(manifest.Entrypoints) != 2 {
		t.Fatalf("Entrypoints = %d, want 2", len(manifest.Entrypoints))
	}
	if got, want := manifest.Entrypoints[0].Name, "turbo"; got != want {
		t.Fatalf("Entrypoints[0].Name = %q, want %q", got, want)
	}
	if got, want := manifest.Entrypoints[0].Group, DefaultEntrypointGroup; got != want {
		t.Fatalf("Entrypoints[0].Group = %q, want %q", got, want)
	}
	if got, want := manifest.Entrypoints[0].Module, "@hotwired/turbo"; got != want {
		t.Fatalf("Entrypoints[0].Module = %q, want %q", got, want)
	}
}

func TestParseManifestSupportsComplexEntrypointProperties(t *testing.T) {
	manifest, err := ParseManifest([]byte(`
package = "frontend/package.json"

[output]
dir = "app/public/vendor"
public_path = "/vendor"
importmap = "app/public/importmap.json"

[bundle]
shared = false
sourcemap = true
target = "es2022"

[entrypoint.monaco]
group = "editors"
module = "monaco-editor/esm/vs/editor/editor.api.js"
imports = ["monaco-editor"]
extra_files = [
  "monaco-editor/esm/vs/editor/editor.worker.js",
  "monaco-editor/esm/vs/language/typescript/ts.worker.js",
]
assets = ["node_modules/monaco-editor/min/vs/**/*"]
`))
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Package != "frontend/package.json" {
		t.Fatalf("Package = %q", manifest.Package)
	}
	if manifest.Output.PublicPath != "/vendor" {
		t.Fatalf("PublicPath = %q", manifest.Output.PublicPath)
	}
	if manifest.Bundle.Shared {
		t.Fatal("Bundle.Shared = true, want false")
	}
	if !manifest.Bundle.Sourcemap {
		t.Fatal("Bundle.Sourcemap = false, want true")
	}

	entrypoint := manifest.Entrypoints[0]
	if got, want := entrypoint.Group, "editors"; got != want {
		t.Fatalf("Group = %q, want %q", got, want)
	}
	if got, want := entrypoint.Imports, []string{"monaco-editor"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Imports = %#v, want %#v", got, want)
	}
	wantExtra := []string{
		"monaco-editor/esm/vs/editor/editor.worker.js",
		"monaco-editor/esm/vs/language/typescript/ts.worker.js",
	}
	if !reflect.DeepEqual(entrypoint.ExtraFiles, wantExtra) {
		t.Fatalf("ExtraFiles = %#v, want %#v", entrypoint.ExtraFiles, wantExtra)
	}
	if got, want := entrypoint.Assets, []string{"node_modules/monaco-editor/min/vs/**/*"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Assets = %#v, want %#v", got, want)
	}
}

func TestParseManifestRequiresEntrypoints(t *testing.T) {
	if _, err := ParseManifest([]byte(`package = "package.json"`)); err == nil {
		t.Fatal("ParseManifest succeeded without entrypoints")
	}
}

func TestFormatManifestOmitsDefaultGroupAndWritesExplicitGroups(t *testing.T) {
	manifest := defaultManifest()
	manifest.Entrypoints = []Entrypoint{
		{
			Name:   "turbo",
			Group:  DefaultEntrypointGroup,
			Module: "@hotwired/turbo",
		},
		{
			Name:    "admin.editor",
			Group:   "admin",
			Module:  "monaco-editor/esm/vs/editor/editor.api.js",
			Imports: []string{"monaco-editor"},
		},
	}

	data, err := FormatManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}

	want := `[entrypoint.turbo]
module = "@hotwired/turbo"

[entrypoint."admin.editor"]
group = "admin"
module = "monaco-editor/esm/vs/editor/editor.api.js"
imports = ["monaco-editor"]
`
	if string(data) != want {
		t.Fatalf("formatted manifest:\n%s\nwant:\n%s", data, want)
	}
}
