package newcommand

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/golazy/lazy/commands"
	"golang.org/x/mod/module"
)

const sampleRepository = "https://github.com/golazy/sample_app"

type Command struct {
	Version string
	Dir     string
	Stdout  io.Writer
	Runner  commands.Runner
}

func (c Command) Execute(modulePath string) error {
	appName, err := applicationName(modulePath)
	if err != nil {
		return err
	}

	dir := c.Dir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}
	destination := filepath.Join(dir, appName)
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("destination %q already exists", appName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect destination %q: %w", appName, err)
	}

	runner := c.Runner
	if runner == nil {
		runner = commands.Exec
	}

	fmt.Fprintln(c.Stdout, "* Initializing the core app")
	if err := runner("git", []string{
		"clone",
		"--branch", c.Version,
		"--depth", "1",
		"--single-branch",
		sampleRepository,
		destination,
	}, commands.Options{Dir: dir, Capture: true}); err != nil {
		return fmt.Errorf("clone sample app at %s: %w", c.Version, err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(destination)
		}
	}()

	if err := os.RemoveAll(filepath.Join(destination, ".git")); err != nil {
		return fmt.Errorf("remove template Git metadata: %w", err)
	}

	fmt.Fprintln(c.Stdout, "* Naming the app")
	if err := replaceTemplateName(destination, modulePath); err != nil {
		return err
	}

	fmt.Fprintln(c.Stdout, "* Validating")
	if err := runner("go", []string{"mod", "tidy"}, commands.Options{
		Dir:     destination,
		Capture: true,
	}); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	if err := runner("go", []string{"test", "./..."}, commands.Options{
		Dir:     destination,
		Capture: true,
	}); err != nil {
		return fmt.Errorf("go test ./...: %w", err)
	}

	cleanup = false
	fmt.Fprintln(c.Stdout, "Congrats !")
	return nil
}

func applicationName(modulePath string) (string, error) {
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return "", fmt.Errorf("module name is required")
	}
	if strings.HasSuffix(modulePath, "/") {
		return "", fmt.Errorf("module name %q has an empty final component", modulePath)
	}
	if err := module.CheckPath(modulePath); err != nil {
		return "", fmt.Errorf("invalid module name %q: %w", modulePath, err)
	}

	name := filepath.Base(modulePath)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "", fmt.Errorf("invalid module name %q", modulePath)
	}
	return name, nil
}

func replaceTemplateName(root, modulePath string) error {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk generated app: %w", err)
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if !utf8.Valid(data) || !bytes.Contains(data, []byte("sample_app")) {
			continue
		}

		updated := bytes.ReplaceAll(data, []byte("sample_app"), []byte(modulePath))
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.WriteFile(path, updated, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
