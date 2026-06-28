package lifecycleservice

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartRunsServicesInParallelAndPreparesAfterChecks(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "postgres/start")
	writeTask(t, dir, "postgres/check")
	writeTask(t, dir, "postgres/create")
	writeTask(t, dir, "postgres/migrate")
	writeTask(t, dir, "minio/start")
	writeTask(t, dir, "minio/check")

	var mu sync.Mutex
	var starts []string
	allStarted := make(chan struct{})
	starter := func(_ string, task string, _ io.Writer, _ io.Writer, _ time.Duration) (Process, error) {
		mu.Lock()
		starts = append(starts, task)
		if len(starts) == 2 {
			close(allStarted)
		}
		mu.Unlock()
		return newFakeProcess(), nil
	}

	var calls []string
	runner := func(_ context.Context, _ string, task string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer, _ bool) error {
		<-allStarted
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, task)
		return nil
	}

	manager, err := (Service{
		Dir:     dir,
		Starter: starter,
		Runner:  runner,
	}).Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()

	if err := waitReady(t, manager); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !slices.Contains(starts, "postgres:start") || !slices.Contains(starts, "minio:start") {
		t.Fatalf("starts = %#v, want postgres and minio", starts)
	}
	assertBefore(t, calls, "postgres:check", "postgres:create")
	assertBefore(t, calls, "postgres:create", "postgres:migrate")
	if !slices.Contains(calls, "minio:check") {
		t.Fatalf("calls = %#v, want minio:check", calls)
	}
}

func TestCheckWarningReportsAfterDelayAndKeepsChecking(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "postgres/start")
	writeTask(t, dir, "postgres/check")

	var stderr bytes.Buffer
	checks := 0
	runner := func(_ context.Context, _ string, task string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer, _ bool) error {
		if task != "postgres:check" {
			t.Fatalf("task = %q, want postgres:check", task)
		}
		checks++
		if checks < 4 {
			return errors.New("not ready")
		}
		return nil
	}

	manager, err := (Service{
		Dir:               dir,
		Starter:           fakeStarter,
		Runner:            runner,
		Stderr:            &stderr,
		CheckInterval:     time.Millisecond,
		CheckWarningDelay: 2 * time.Millisecond,
	}).Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()

	if err := waitReady(t, manager); err != nil {
		t.Fatal(err)
	}
	if checks < 4 {
		t.Fatalf("checks = %d, want repeated checks", checks)
	}
	output := stderr.String()
	if !strings.Contains(output, "postgres:check is still failing after 2ms") {
		t.Fatalf("stderr = %q, want delayed warning", output)
	}
	if !strings.Contains(output, "lazy will keep checking") {
		t.Fatalf("stderr = %q, want keep checking note", output)
	}
}

func TestCreateFailureIsReportedAndMigrateStillRuns(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "postgres/start")
	writeTask(t, dir, "postgres/check")
	writeTask(t, dir, "postgres/create")
	writeTask(t, dir, "postgres/migrate")

	var stderr bytes.Buffer
	var calls []string
	runner := func(_ context.Context, _ string, task string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer, _ bool) error {
		calls = append(calls, task)
		if task == "postgres:create" {
			return errors.New("database exists but task is not idempotent")
		}
		return nil
	}

	manager, err := (Service{
		Dir:     dir,
		Starter: fakeStarter,
		Runner:  runner,
		Stderr:  &stderr,
	}).Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()

	if err := waitReady(t, manager); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(calls, "postgres:migrate") {
		t.Fatalf("calls = %#v, want migrate after create failure", calls)
	}
	if !strings.Contains(stderr.String(), "postgres:create failed") {
		t.Fatalf("stderr = %q, want create failure", stderr.String())
	}
}

func waitReady(t *testing.T, manager *Manager) error {
	t.Helper()
	select {
	case err := <-manager.Ready():
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for lifecycle readiness")
		return nil
	}
}

func assertBefore(t *testing.T, values []string, before string, after string) {
	t.Helper()
	beforeIndex := slices.Index(values, before)
	afterIndex := slices.Index(values, after)
	if beforeIndex == -1 || afterIndex == -1 {
		t.Fatalf("values = %#v, want %s before %s", values, before, after)
	}
	if beforeIndex > afterIndex {
		t.Fatalf("values = %#v, want %s before %s", values, before, after)
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

func fakeStarter(_ string, _ string, _ io.Writer, _ io.Writer, _ time.Duration) (Process, error) {
	return newFakeProcess(), nil
}

type fakeProcess struct {
	done         chan error
	completeOnce sync.Once
}

func newFakeProcess() *fakeProcess {
	return &fakeProcess{done: make(chan error, 1)}
}

func (p *fakeProcess) Done() <-chan error {
	return p.done
}

func (p *fakeProcess) Stop() {
	p.completeOnce.Do(func() {
		p.done <- nil
		close(p.done)
	})
}

func (p *fakeProcess) Kill() {
	p.completeOnce.Do(func() {
		p.done <- errors.New("killed")
		close(p.done)
	})
}
