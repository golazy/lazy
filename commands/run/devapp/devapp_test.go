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
)

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
