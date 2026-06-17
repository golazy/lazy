package jscommand

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEnsurePackageDependenciesCreatesPackageJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")

	changed, err := ensurePackageDependencies(path, []string{"@hotwired/stimulus", "@hotwired/turbo"})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}

	document := readJSONFile(t, path)
	if document["private"] != true {
		t.Fatalf("private = %#v, want true", document["private"])
	}
	dependencies := document["dependencies"].(map[string]any)
	if dependencies["@hotwired/stimulus"] != "latest" {
		t.Fatalf("stimulus dependency = %#v", dependencies["@hotwired/stimulus"])
	}
	if dependencies["@hotwired/turbo"] != "latest" {
		t.Fatalf("turbo dependency = %#v", dependencies["@hotwired/turbo"])
	}
}

func TestEnsurePackageDependenciesKeepsExistingDependencyVersions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	writeFile(t, path, `{
  "private": true,
  "dependencies": {
    "@hotwired/turbo": "^8.0.0"
  },
  "devDependencies": {
    "@hotwired/stimulus": "^3.0.0"
  }
}`)

	changed, err := ensurePackageDependencies(path, []string{"@hotwired/stimulus", "@hotwired/turbo"})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
}

func TestRequiredPackagesUsesPackageNamesFromModuleSpecifiers(t *testing.T) {
	manifest := Manifest{
		Entrypoints: []Entrypoint{
			{
				Module:     "@hotwired/turbo/dist/turbo.es2017-esm.js",
				ExtraFiles: []string{"monaco-editor/esm/vs/editor/editor.worker.js", "./local-worker.js"},
			},
		},
	}

	got := requiredPackages(manifest)
	want := []string{"@hotwired/turbo", "monaco-editor"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requiredPackages = %#v, want %#v", got, want)
	}
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
