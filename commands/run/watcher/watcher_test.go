package watcher

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestWatchPathIncludesRuntimeInputs(t *testing.T) {
	tests := map[string]bool{
		"app/controllers/home.go":                true,
		"app/js/app.js":                          true,
		"app/js/controllers/hello_controller.js": true,
		"services/posts/content/a.md":            true,
		"app/views/home/index.html.tpl":          true,
		"cmd/app/main.go":                        true,
		"go.mod":                                 true,
		"js.toml":                                true,
		"public/styles.css":                      true,
		"views/pages/index.html.tpl":             true,
		"README.md":                              false,
		"app/controllers/home_test.go":           false,
		"node_modules/pkg/index.js":              false,
		"app/views/home/index.html.tpl~":         false,
	}
	for path, want := range tests {
		if got := watchPath(path); got != want {
			t.Fatalf("watchPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestDiffSnapshotsReportsCreatesWritesAndRemoves(t *testing.T) {
	earlier := time.Unix(10, 0)
	later := time.Unix(20, 0)
	previous := map[string]watchedFile{
		"app/controllers/home.go": {modTime: earlier, size: 10},
		"app/views/old.html.tpl":  {modTime: earlier, size: 20},
		"go.mod":                  {modTime: earlier, size: 30},
	}
	next := map[string]watchedFile{
		"app/controllers/home.go": {modTime: later, size: 10},
		"app/views/new.html.tpl":  {modTime: earlier, size: 20},
		"go.mod":                  {modTime: earlier, size: 30},
	}

	got := diffSnapshots(previous, next)
	want := []string{"app/controllers/home.go", "app/views/new.html.tpl", "app/views/old.html.tpl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffSnapshots() = %#v, want %#v", got, want)
	}
}

func TestDiffSnapshotsIgnoresMTimeOnlyPackageInputChanges(t *testing.T) {
	dir := t.TempDir()
	lockfile := filepath.Join(dir, "package-lock.json")
	writeFile(t, lockfile, "{\"lockfileVersion\":3}\n")

	previous, err := scanWatchedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(lockfile, time.Now().Add(time.Second), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	next, err := scanWatchedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := diffSnapshots(previous, next); len(got) != 0 {
		t.Fatalf("diffSnapshots() = %#v, want no package-lock mtime-only change", got)
	}
}

func TestDiffSnapshotsReportsSameSizePackageInputContentChanges(t *testing.T) {
	dir := t.TempDir()
	lockfile := filepath.Join(dir, "package-lock.json")
	writeFile(t, lockfile, "aaaa\n")

	previous, err := scanWatchedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, lockfile, "bbbb\n")
	next, err := scanWatchedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	got := diffSnapshots(previous, next)
	want := []string{"package-lock.json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffSnapshots() = %#v, want %#v", got, want)
	}
}

func TestScanWatchedFilesSkipsIgnoredDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "controllers", "home.go"), "package controllers\n")
	writeFile(t, filepath.Join(dir, "node_modules", "pkg", "index.js"), "console.log('x')\n")
	writeFile(t, filepath.Join(dir, "README.md"), "notes\n")

	files, err := scanWatchedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["app/controllers/home.go"]; !ok {
		t.Fatalf("watched files = %#v, want app controller", files)
	}
	if _, ok := files["node_modules/pkg/index.js"]; ok {
		t.Fatalf("watched files includes node_modules: %#v", files)
	}
	if _, ok := files["README.md"]; ok {
		t.Fatalf("watched files includes README.md: %#v", files)
	}
}

func writeFile(t *testing.T, filename string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
