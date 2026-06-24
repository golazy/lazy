package devapp

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golazy.dev/lazy/commands/appcmd"
)

func TestBuildRunsGoModTidyBeforeBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake go command is POSIX-only")
	}

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	writeTestFile(t, filepath.Join(dir, "app", "public", ".keep"), "")
	fakeBin := filepath.Join(dir, "bin")
	logPath := filepath.Join(dir, "go.log")
	writeTestFile(t, filepath.Join(fakeBin, "mise"), `#!/bin/sh
if [ "$1" = "exec" ] && [ "$2" = "--" ]; then
  shift 2
  exec "$@"
fi
exit 1
`)
	writeTestFile(t, filepath.Join(fakeBin, "go"), `#!/bin/sh
if [ "$1" = "env" ] && [ "$2" = "GOWORK" ]; then
  if [ -n "$FAKE_GOWORK" ]; then
    printf '%s\n' "$FAKE_GOWORK"
  fi
  exit 0
fi
printf '%s\n' "$*" >> "$GO_LOG"
if [ "$1" = "mod" ] && [ "$2" = "tidy" ]; then
  exit 0
fi
if [ "$1" = "build" ]; then
  output=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "-o" ]; then
      shift
      output=$1
    fi
    shift || true
  done
  printf 'built\n' > "$output"
  exit 0
fi
exit 1
`)
	if err := os.Chmod(filepath.Join(fakeBin, "mise"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(fakeBin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_LOG", logPath)

	result := (Config{
		Root:        dir,
		CommandPath: "cmd/app",
	}).Build(context.Background(), dir, 1)
	if result.Err != nil {
		t.Fatalf("build failed: %v\n%s", result.Err, result.Output)
	}
	if _, err := os.Stat(result.Binary); err != nil {
		t.Fatalf("built binary does not exist: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "mod tidy\nbuild -tags lazydev -ldflags " + appcmd.LazyDevLDFlags(appcmd.LazyDevPaths{
		Views:  filepath.Join(dir, "app", "views"),
		Public: filepath.Join(dir, "app", "public"),
	}) + " -o " + result.Binary + " ./cmd/app\n"
	if string(data) != want {
		t.Fatalf("go log = %q, want %q", data, want)
	}
}

func TestBuildSkipsGoModTidyWhenGoEnvGOWORKIsActive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake go command is POSIX-only")
	}

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	writeTestFile(t, filepath.Join(dir, "app", "public", ".keep"), "")
	fakeBin := filepath.Join(dir, "bin")
	logPath := filepath.Join(dir, "go.log")
	writeTestFile(t, filepath.Join(fakeBin, "go"), `#!/bin/sh
if [ "$1" = "env" ] && [ "$2" = "GOWORK" ]; then
  printf '%s\n' "$FAKE_GOWORK"
  exit 0
fi
printf '%s\n' "$*" >> "$GO_LOG"
if [ "$1" = "build" ]; then
  output=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "-o" ]; then
      shift
      output=$1
    fi
    shift || true
  done
  printf 'built\n' > "$output"
  exit 0
fi
exit 1
`)
	if err := os.Chmod(filepath.Join(fakeBin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_LOG", logPath)
	t.Setenv("FAKE_GOWORK", filepath.Join(dir, "go.work"))
	t.Setenv("GOWORK", "")

	result := (Config{
		Root:        dir,
		CommandPath: "cmd/app",
	}).Build(context.Background(), dir, 1)
	if result.Err != nil {
		t.Fatalf("build failed: %v\n%s", result.Err, result.Output)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "build -tags lazydev -ldflags " + appcmd.LazyDevLDFlags(appcmd.LazyDevPaths{
		Views:  filepath.Join(dir, "app", "views"),
		Public: filepath.Join(dir, "app", "public"),
	}) + " -o " + result.Binary + " ./cmd/app\n"
	if string(data) != want {
		t.Fatalf("go log = %q, want %q", data, want)
	}
}

func TestContextCancellationInterruptsApplication(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt process signaling is not reliable on Windows")
	}

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "app", "views", "layouts", "app.html.tpl"), "layout")
	binary := buildSignalApp(t, dir)
	marker := filepath.Join(dir, "stopped")
	t.Setenv("LAZY_DEVAPP_STOP_MARKER", marker)

	ctx, cancel := context.WithCancel(context.Background())
	process, err := (Config{
		Root:           dir,
		Stdout:         io.Discard,
		Stderr:         io.Discard,
		StartupTimeout: 2 * time.Second,
		StopTimeout:    2 * time.Second,
	}).Start(ctx, binary)
	if err != nil {
		t.Fatal(err)
	}

	cancel()
	select {
	case <-process.Done():
	case <-time.After(3 * time.Second):
		process.Stop()
		t.Fatal("application did not exit after context cancellation")
	}

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("application did not handle interrupt before exiting: %v", err)
	}
}

func writeTestFile(t *testing.T, filename string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildSignalApp(t *testing.T, dir string) string {
	t.Helper()

	source := filepath.Join(dir, "signal_app.go")
	binary := filepath.Join(dir, "signal-app")
	if err := os.WriteFile(source, []byte(signalAppSource), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "build", "-o", binary, source)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build signal app: %v\n%s", err, output)
	}
	return binary
}

const signalAppSource = `package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	listener, err := net.Listen("tcp", os.Getenv("ADDR"))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	time.Sleep(100 * time.Millisecond)
	if marker := os.Getenv("LAZY_DEVAPP_STOP_MARKER"); marker != "" {
		if err := os.WriteFile(marker, []byte("stopped\n"), 0o644); err != nil {
			panic(err)
		}
	}
}
`
