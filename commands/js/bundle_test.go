package jscommand

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundleWritesImportmapAndCopiesAssets(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.js"), "export const value = 42;\n")
	writeFile(t, filepath.Join(dir, "worker.js"), "self.postMessage('ready');\n")
	writeFile(t, filepath.Join(dir, "assets", "nested", "icon.txt"), "icon\n")

	manifest := defaultManifest()
	manifest.Package = "package.json"
	manifest.Bundle.Minify = false
	manifest.Entrypoints = []Entrypoint{
		{
			Name:       "library",
			Module:     "./lib.js",
			Imports:    []string{"library"},
			ExtraFiles: []string{"./worker.js"},
			Assets:     []string{"assets/**/*"},
		},
	}

	result, err := Bundle(manifest, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imports["library"] == "" {
		t.Fatalf("result imports = %#v", result.Imports)
	}
	if !strings.HasPrefix(result.Imports["library"], "/assets/lazyshaft/library-") {
		t.Fatalf("library import = %q", result.Imports["library"])
	}

	importmap := readImportmap(t, filepath.Join(dir, "app", "public", "assets", "importmap.json"))
	if importmap.Imports["library"] != result.Imports["library"] {
		t.Fatalf("importmap imports = %#v, want %#v", importmap.Imports, result.Imports)
	}

	outputs := listFiles(t, filepath.Join(dir, "app", "public", "assets", "lazyshaft"))
	if !containsPrefix(outputs, "library-") {
		t.Fatalf("outputs = %#v, want library bundle", outputs)
	}
	if !containsPrefix(outputs, "library-worker-") {
		t.Fatalf("outputs = %#v, want worker bundle", outputs)
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "public", "assets", "lazyshaft", "assets", "nested", "icon.txt")); err != nil {
		t.Fatalf("copied asset missing: %v", err)
	}
}

func TestBundleSharesChunksWithinEntrypointGroups(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "shared.js"), "export const value = 42;\n")
	writeFile(t, filepath.Join(dir, "one.js"), "import { value } from './shared.js'; console.log('one', value);\n")
	writeFile(t, filepath.Join(dir, "two.js"), "import { value } from './shared.js'; console.log('two', value);\n")

	manifest := defaultManifest()
	manifest.Bundle.Minify = false
	manifest.Entrypoints = []Entrypoint{
		{Name: "one", Module: "./one.js"},
		{Name: "two", Module: "./two.js"},
	}

	if _, err := Bundle(manifest, dir, dir); err != nil {
		t.Fatal(err)
	}
	outputs := listFiles(t, filepath.Join(dir, "app", "public", "assets", "lazyshaft"))
	if !containsPrefix(outputs, "shared-") {
		t.Fatalf("outputs = %#v, want shared chunk for matching group", outputs)
	}

	manifest.Entrypoints[0].Group = "admin"
	manifest.Entrypoints[1].Group = "public"
	if _, err := Bundle(manifest, dir, dir); err != nil {
		t.Fatal(err)
	}
	outputs = listFiles(t, filepath.Join(dir, "app", "public", "assets", "lazyshaft"))
	if containsPrefix(outputs, "shared-") {
		t.Fatalf("outputs = %#v, want no shared chunk across different groups", outputs)
	}
}

func readImportmap(t *testing.T, path string) importMap {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed importMap
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed
}

func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func containsPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
