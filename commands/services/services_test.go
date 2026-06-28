package services

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/lazyconfig"
)

func TestInspectUsesConfiguredServicesBeforeDiscoveredTasks(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "redis/start")
	writeTask(t, dir, "postgres/start")

	inventory, err := Inspect(dir, lazyconfig.Config{
		Services: []lazyconfig.Service{{Name: "db"}, {Name: "search"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inventory.Services, []string{"db", "search"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %#v, want %#v", got, want)
	}
}

func TestInspectDiscoversStartTasksWhenConfigHasNoServices(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "postgres/start")
	writeTask(t, dir, "s3/start.go")
	writeTask(t, dir, "secrets/_lib.sh")
	writeTask(t, dir, "postgres/migrate")

	inventory, err := Inspect(dir, lazyconfig.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inventory.Services, []string{"postgres", "s3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %#v, want %#v", got, want)
	}
	if !HasTask(inventory.Tasks, "postgres", "migrate") {
		t.Fatalf("postgres:migrate was not discovered")
	}
	if HasTask(inventory.Tasks, "secrets", "_lib") {
		t.Fatalf("hidden support task was discovered")
	}
}

func TestPreparerRunsOptionalLifecycleTasksInOrder(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "db/start")
	writeTask(t, dir, "db/check")
	writeTask(t, dir, "db/create")
	writeTask(t, dir, "db/migrate")

	var calls []string
	runner := func(command string, args []string, options commands.Options) error {
		calls = append(calls, args[1])
		return nil
	}

	err := (Preparer{
		Dir:           dir,
		Runner:        runner,
		CheckTimeout:  time.Millisecond,
		CheckInterval: time.Nanosecond,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"db:check", "db:create", "db:migrate"}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestPreparerReturnsCheckTimeout(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "db/start")
	writeTask(t, dir, "db/check")

	err := (Preparer{
		Dir: dir,
		Runner: func(string, []string, commands.Options) error {
			return errors.New("not ready")
		},
		CheckTimeout:  time.Nanosecond,
		CheckInterval: time.Nanosecond,
	}).Execute()
	if err == nil {
		t.Fatal("err = nil, want timeout")
	}
}

func writeTask(t *testing.T, dir string, name string) {
	t.Helper()
	path := filepath.Join(dir, ".mise", "tasks", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
