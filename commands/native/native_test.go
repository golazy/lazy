package native

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"golazy.dev/lazy/commands"
)

type nativeInvocation struct {
	command string
	args    []string
	dir     string
	env     []string
}

func TestDevRunsCachedNativeHelper(t *testing.T) {
	cacheDir := t.TempDir()
	helper := writeCachedHelper(t, cacheDir, "abc123", "linux")
	listener := fixedListener{addr: fixedAddr("127.0.0.1:4455")}

	var calls []nativeInvocation
	command := Command{
		Dir:          "/workspace/shop",
		CmdPath:      "cmd/web",
		ViewPath:     "views",
		PublicPath:   "public_files",
		Title:        "Shop",
		Width:        1200,
		Height:       800,
		Stdin:        strings.NewReader(""),
		Stdout:       &bytes.Buffer{},
		Stderr:       &bytes.Buffer{},
		UserCacheDir: func() (string, error) { return cacheDir, nil },
		OutputRunner: func(command string, args []string, options commands.Options) ([]byte, error) {
			return []byte("abc123\tHEAD\n"), nil
		},
		Runner: func(command string, args []string, options commands.Options) error {
			calls = append(calls, nativeInvocation{
				command: command,
				args:    slices.Clone(args),
				dir:     options.Dir,
				env:     slices.Clone(options.Env),
			})
			return nil
		},
		Executable: func() (string, error) { return "/usr/local/bin/lazy", nil },
		Listen: func(network string, address string) (net.Listener, error) {
			if network != "tcp" || address != "127.0.0.1:0" {
				t.Fatalf("listen(%q, %q), want tcp 127.0.0.1:0", network, address)
			}
			return listener, nil
		},
		GOOS:   "linux",
		GOARCH: "amd64",
	}

	code, err := command.ExecuteDev()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	wantArgs := []string{
		"--dir", "/workspace/shop",
		"--addr", "127.0.0.1:4455",
		"--env", "ADDR=127.0.0.1:4455",
		"--env", "LAZY_TMUX=1",
		"--env", "LAZY_MULTIVERSION=off",
		"--title", "Shop",
		"--width", "1200",
		"--height", "800",
		"--", "/usr/local/bin/lazy",
		"--cmdpath", "cmd/web",
		"--viewpath", "views",
		"--publicpath", "public_files",
	}
	if got, want := calls, []nativeInvocation{{
		command: helper,
		args:    wantArgs,
		dir:     "/workspace/shop",
	}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestDevFallsBackToCachedHelperWhenLatestCommitIsUnavailable(t *testing.T) {
	cacheDir := t.TempDir()
	helper := writeCachedHelper(t, cacheDir, "cached", "linux")

	var calls []nativeInvocation
	command := Command{
		Dir:          "/workspace/shop",
		UserCacheDir: func() (string, error) { return cacheDir, nil },
		OutputRunner: func(command string, args []string, options commands.Options) ([]byte, error) {
			return nil, errors.New("network unavailable")
		},
		Runner: func(command string, args []string, options commands.Options) error {
			calls = append(calls, nativeInvocation{command: command, args: slices.Clone(args), dir: options.Dir})
			return nil
		},
		Executable: func() (string, error) { return "/usr/local/bin/lazy", nil },
		Listen: func(network string, address string) (net.Listener, error) {
			return fixedListener{addr: fixedAddr("127.0.0.1:4455")}, nil
		},
		GOOS:   "linux",
		GOARCH: "amd64",
	}

	code, err := command.ExecuteDev()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(calls) != 1 || calls[0].command != helper {
		t.Fatalf("calls = %#v, want cached helper %q", calls, helper)
	}
}

func TestBuildBuildsApplicationAndRunsNativeHelperForCurrentPlatform(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/shop\n")
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), `package main

import _ "golazy.dev/lazyapp"

func main() {}
`)

	cacheDir := t.TempDir()
	helper := writeCachedHelper(t, cacheDir, "abc123", "linux")
	workDir := filepath.Join(t.TempDir(), "work")

	var calls []nativeInvocation
	command := Command{
		Dir:          dir,
		UserCacheDir: func() (string, error) { return cacheDir, nil },
		TempDir: func(string, string) (string, error) {
			if err := os.MkdirAll(workDir, 0o755); err != nil {
				t.Fatal(err)
			}
			return workDir, nil
		},
		OutputRunner: func(command string, args []string, options commands.Options) ([]byte, error) {
			return []byte("abc123\tHEAD\n"), nil
		},
		Runner: func(command string, args []string, options commands.Options) error {
			calls = append(calls, nativeInvocation{command: command, args: slices.Clone(args), dir: options.Dir})
			return nil
		},
		GOOS:   "linux",
		GOARCH: "amd64",
	}

	code, err := command.ExecuteBuild()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	appBinary := filepath.Join(workDir, "app")
	want := []nativeInvocation{
		{
			command: "go",
			args:    []string{"build", "-o", appBinary, "./cmd/app"},
			dir:     dir,
		},
		{
			command: helper,
			args: []string{
				"build",
				"--dir", dir,
				"--target", "linux-amd64",
				"--app-binary", appBinary,
				"--out", filepath.Join("dist", "native", "linux-amd64"),
			},
			dir: dir,
		},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestBuildRejectsNonCurrentTarget(t *testing.T) {
	code, err := (Command{
		Dir:    t.TempDir(),
		Target: "darwin",
		GOOS:   "linux",
		GOARCH: "amd64",
	}).ExecuteBuild()
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "not the current platform linux-amd64") {
		t.Fatalf("err = %v, want current platform error", err)
	}
}

func writeCachedHelper(t *testing.T, cacheDir string, commit string, goos string) string {
	t.Helper()
	helper := filepath.Join(cacheDir, "golazy", "native", "builds", commit, helperName(goos))
	if err := os.MkdirAll(filepath.Dir(helper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return helper
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

type fixedListener struct {
	addr net.Addr
}

func (f fixedListener) Accept() (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (f fixedListener) Close() error {
	return nil
}

func (f fixedListener) Addr() net.Addr {
	return f.addr
}

type fixedAddr string

func (f fixedAddr) Network() string {
	return "tcp"
}

func (f fixedAddr) String() string {
	return string(f)
}
