package datasets

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"golazy.dev/lazy/commands"
)

func TestDumpRunsServiceDumpTasksToDatasetFiles(t *testing.T) {
	dir := t.TempDir()
	writeDatasetTask(t, dir, "postgres/start")
	writeDatasetTask(t, dir, "postgres/dump")
	writeDatasetTask(t, dir, "minio/start")

	var calls []call
	err := (Command{
		Dir: dir,
		Runner: func(command string, args []string, options commands.Options) error {
			calls = append(calls, call{command: command, args: slices.Clone(args), dir: options.Dir})
			return nil
		},
	}).Dump("baseline")
	if err != nil {
		t.Fatal(err)
	}

	wantPath := filepath.Join(dir, "datasets", "baseline", "postgres.dump")
	want := []call{{
		command: "mise",
		args:    []string{"run", "postgres:dump", "--", wantPath},
		dir:     dir,
	}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
	if info, err := os.Stat(filepath.Join(dir, "datasets", "baseline")); err != nil || !info.IsDir() {
		t.Fatalf("dataset directory not created: info=%v err=%v", info, err)
	}
}

func TestLoadRunsServiceLoadTasksForExistingDumpFiles(t *testing.T) {
	dir := t.TempDir()
	writeDatasetTask(t, dir, "postgres/start")
	writeDatasetTask(t, dir, "postgres/load")
	datasetDir := filepath.Join(dir, "datasets", "baseline")
	if err := os.MkdirAll(datasetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(datasetDir, "postgres.dump"), []byte("dump"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls []call
	err := (Command{
		Dir: dir,
		Runner: func(command string, args []string, options commands.Options) error {
			calls = append(calls, call{command: command, args: slices.Clone(args), dir: options.Dir})
			return nil
		},
	}).Load("baseline")
	if err != nil {
		t.Fatal(err)
	}

	want := []call{{
		command: "mise",
		args:    []string{"run", "postgres:load", "--", filepath.Join(datasetDir, "postgres.dump")},
		dir:     dir,
	}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestLoadRejectsDatasetTraversal(t *testing.T) {
	err := (Command{Dir: t.TempDir()}).Load("../baseline")
	if err == nil {
		t.Fatal("err = nil, want invalid dataset name")
	}
}

type call struct {
	command string
	args    []string
	dir     string
}

func writeDatasetTask(t *testing.T, dir string, name string) {
	t.Helper()
	path := filepath.Join(dir, ".mise", "tasks", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
