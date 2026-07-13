package addonservice

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golazy.dev/lazycode"
)

func TestApplyRollsBackEveryOutputWhenInstallingStateFails(t *testing.T) {
	dir := t.TempDir()
	goMod := "module example.com/rollback\n\ngo 1.26.2\n\nrequire golazy.dev v0.1.20\n"
	app := `package appinit

import "golazy.dev/lazyapp"

func App() *lazyapp.App { return lazyapp.New(lazyapp.Config{Name: "rollback"}) }
`
	writeInternalFile(t, dir, GoModFilename, goMod)
	writeInternalFile(t, dir, ApplicationFilename, app)
	plan, err := (Manager{Dir: dir, Sources: []Source{FirstPartySource("v0.1.21")}}).Add("postgres")
	if err != nil {
		t.Fatal(err)
	}

	originalRename := renameAddonFile
	renameAddonFile = func(oldPath, newPath string) error {
		if filepath.Base(newPath) == StateFilename {
			return errors.New("injected state install failure")
		}
		return os.Rename(oldPath, newPath)
	}
	t.Cleanup(func() { renameAddonFile = originalRename })

	err = plan.Apply()
	if err == nil || !strings.Contains(err.Error(), "injected state install failure") {
		t.Fatalf("Apply error = %v", err)
	}
	if got := readInternalFile(t, dir, GoModFilename); got != goMod {
		t.Fatalf("go.mod was not rolled back:\n%s", got)
	}
	if got := readInternalFile(t, dir, ApplicationFilename); got != app {
		t.Fatalf("init/app.go was not rolled back:\n%s", got)
	}
	for _, name := range []string{
		StateFilename,
		GeneratedFilename,
		lazyConfigFilename,
		miseConfigFilename,
		postgresMigrationsGo,
		postgresMigrationsIndex,
		".mise/tasks/postgres/start",
	} {
		if _, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("%s survived rollback: %v", name, statErr)
		}
	}
}

func TestApplyRejectsTargetThroughSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "app", "services")); err != nil {
		t.Fatal(err)
	}
	plan := Plan{
		Dir: root,
		Result: lazycode.Result{Files: []lazycode.FileEdit{{
			Path:         "app/services/escaped.go",
			Kind:         lazycode.EditCreate,
			BaselineHash: lazycode.AbsentHash,
			After:        []byte("package services\n"),
		}}},
	}
	if err := plan.Apply(); err == nil || !strings.Contains(err.Error(), "outside the application root") {
		t.Fatalf("Apply error = %v, want target-containment error", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside target was written: %v", err)
	}
}

func writeInternalFile(t *testing.T, root, name, data string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readInternalFile(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
