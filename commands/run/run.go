package run

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/golazy/lazy/commands"
	"golang.org/x/mod/modfile"
)

type Command struct {
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Runner commands.Runner
}

func (c Command) Execute() (int, error) {
	dir := c.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return 1, fmt.Errorf("get working directory: %w", err)
		}
	}

	moduleName, err := moduleName(filepath.Join(dir, "go.mod"))
	if err != nil {
		return 1, err
	}

	appName := filepath.Base(moduleName)
	candidates := []string{
		filepath.Join("cmd", appName),
		filepath.Join("cmd", "app"),
	}

	for _, candidate := range candidates {
		if !isDirectory(filepath.Join(dir, candidate)) {
			continue
		}

		runner := c.Runner
		if runner == nil {
			runner = commands.Exec
		}
		err := runner("go", []string{"run", "./" + filepath.ToSlash(candidate)}, commands.Options{
			Dir:    dir,
			Stdin:  c.Stdin,
			Stdout: c.Stdout,
			Stderr: c.Stderr,
		})
		if err == nil {
			return 0, nil
		}

		var processExit *commands.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, fmt.Errorf("run application: %w", err)
	}

	return 1, fmt.Errorf(
		"application command not found; tried ./cmd/%s and ./cmd/app",
		appName,
	)
}

func moduleName(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("go.mod not found")
		}
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	module := modfile.ModulePath(data)
	if module == "" {
		return "", fmt.Errorf("go.mod does not declare a module")
	}
	return module, nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
