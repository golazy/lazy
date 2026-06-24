package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/modfile"
)

var (
	userCacheDir = os.UserCacheDir
	executable   = os.Executable
	statFile     = os.Stat
	mkdirAll     = os.MkdirAll
	runCommand   = runExternalCommand
)

func maybeExecuteLazyCmd(config envConfig, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (bool, int) {
	target := config.lazyCmdTarget()
	if target == "" {
		return false, 0
	}

	same, targetPath, err := lazyCmdMatchesCurrentExecutable(target)
	if err != nil {
		fmt.Fprintf(stderr, "lazy: inspect %s: %v\n", target, err)
		return true, 1
	}
	if same {
		return false, 0
	}

	fmt.Fprintf(stderr, "Running %s instead\n", targetPath)
	code, err := runCommand(targetPath, args, commandOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Env:    []string{lazyMultiversionEnv + "=" + lazyMultiversionOff},
	})
	if err != nil {
		fmt.Fprintf(stderr, "lazy: run %s: %v\n", targetPath, err)
		return true, 1
	}
	return true, code
}

func lazyCmdMatchesCurrentExecutable(target string) (bool, string, error) {
	targetPath, err := canonicalExecutablePath(target)
	if err != nil {
		return false, "", err
	}
	current, err := executable()
	if err != nil {
		return false, targetPath, fmt.Errorf("current executable: %w", err)
	}
	currentPath, err := canonicalExecutablePath(current)
	if err != nil {
		return false, targetPath, fmt.Errorf("current executable: %w", err)
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(currentPath, targetPath), targetPath, nil
	}
	return currentPath == targetPath, targetPath, nil
}

func canonicalExecutablePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	return absolute, nil
}

func maybeExecuteProjectVersion(config envConfig, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (bool, int) {
	if skipProjectVersionCheck(config, args) {
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

func skipProjectVersionCheck(config envConfig, args []string) bool {
	if config.multiversionOff() {
		return true
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
		Env:    []string{lazyMultiversionEnv + "=" + lazyMultiversionOff},
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
