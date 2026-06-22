package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"golang.org/x/mod/modfile"
)

const (
	noVersionCheckEnv    = "NO_VERSION_CHECK"
	skipVersionCheckFlag = "--skip-version-check"
)

var (
	userCacheDir = os.UserCacheDir
	statFile     = os.Stat
	mkdirAll     = os.MkdirAll
	runCommand   = runExternalCommand
)

func maybeExecuteProjectVersion(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (bool, int) {
	if skipProjectVersionCheck(args) {
		return false, 0
	}

	version, ok, err := projectGoLazyVersion("go.mod")
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
		return true, 1
	}
	if !ok || version == currentVersion() {
		return false, 0
	}

	binary, err := lazyVersionBinary(version)
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
		return true, 1
	}

	if _, err := statFile(binary); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "lazy: inspect %s: %v\n", binary, err)
			return true, 1
		}
		if err := installLazyVersion(version, filepath.Dir(binary), stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "lazy: install golazy.dev/lazy@%s: %v\n", version, err)
			return true, 1
		}
	}

	code, err := runLazyVersion(binary, args, stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "lazy: run %s: %v\n", binary, err)
		return true, 1
	}
	return true, code
}

func skipProjectVersionCheck(args []string) bool {
	if os.Getenv(noVersionCheckEnv) == "true" {
		return true
	}
	for _, arg := range args {
		if arg == skipVersionCheckFlag {
			return true
		}
	}
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "--version", "new", "upgrade", "command-center", "bastard":
		return true
	default:
		return false
	}
}

func projectGoLazyVersion(filename string) (string, bool, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read go.mod: %w", err)
	}

	file, err := modfile.Parse(filename, data, nil)
	if err != nil {
		return "", false, fmt.Errorf("parse go.mod: %w", err)
	}
	for _, require := range file.Require {
		if require.Mod.Path == "golazy.dev" {
			return require.Mod.Version, true, nil
		}
	}
	return "", false, nil
}

func lazyVersionBinary(version string) (string, error) {
	cacheDir, err := userCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	name := "lazy"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(cacheDir, "golazy", "lazy", "builds", version, name), nil
}

func installLazyVersion(version string, binDir string, stdout io.Writer, stderr io.Writer) error {
	if err := mkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", binDir, err)
	}
	code, err := runCommand("go", []string{"install", "golazy.dev/lazy@" + version}, commandOptions{
		Stdout: stdout,
		Stderr: stderr,
		Env:    []string{"GOBIN=" + binDir},
	})
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("go install exited with code %d", code)
	}
	return nil
}

func runLazyVersion(binary string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int, error) {
	return runCommand(binary, args, commandOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Env:    []string{noVersionCheckEnv + "=true"},
	})
}

type commandOptions struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    []string
}

func runExternalCommand(command string, args []string, options commandOptions) (int, error) {
	process := exec.Command(command, args...)
	process.Stdin = options.Stdin
	process.Stdout = options.Stdout
	process.Stderr = options.Stderr
	if len(options.Env) != 0 {
		process.Env = append(os.Environ(), options.Env...)
	}
	if err := process.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return exitError.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
