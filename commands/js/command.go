package jscommand

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/mise"
)

type Bundler func(manifest Manifest, root, packageDir string) (BuildResult, error)

type Command struct {
	Dir     string
	Stdout  io.Writer
	Stderr  io.Writer
	Runner  commands.Runner
	Mise    commands.OutputRunner
	Bundler Bundler
}

func (c Command) Execute() (int, error) {
	stdout := c.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := c.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	dir := c.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return 1, fmt.Errorf("get working directory: %w", err)
		}
	}

	root, err := findAppRoot(dir)
	if err != nil {
		return 1, err
	}

	manifest, err := LoadManifest(root)
	if err != nil {
		return 1, err
	}

	packagePath := resolvePath(root, manifest.Package)
	packageDir := filepath.Dir(packagePath)
	fmt.Fprintln(stdout, "* Preparing JavaScript dependencies")
	if _, err := ensurePackageDependencies(packagePath, requiredPackages(manifest)); err != nil {
		return 1, err
	}

	runner := c.Runner
	if runner == nil {
		runner = commands.Exec
	}
	packageManager := mise.DetectNodePackageManager(packageDir, mise.QueryRunner(c.Runner, c.Mise))
	runCommand, runArgs, runEnv := packageManager.InstallCommand(c.Runner)
	fmt.Fprintln(stdout, "* Installing JavaScript dependencies")
	if err := runner(runCommand, runArgs, commands.Options{
		Dir:    packageDir,
		Env:    runEnv,
		Stdout: stdout,
		Stderr: stderr,
	}); err != nil {
		var processExit *commands.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, fmt.Errorf("%s %v: %w", runCommand, runArgs, err)
	}

	bundler := c.Bundler
	if bundler == nil {
		bundler = Bundle
	}
	fmt.Fprintln(stdout, "* Bundling JavaScript")
	if _, err := bundler(manifest, root, packageDir); err != nil {
		return 1, err
	}

	return 0, nil
}

func findAppRoot(start string) (string, error) {
	current := start
	for {
		candidate := filepath.Join(current, "go.mod")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return current, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect %s: %w", candidate, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("go.mod not found")
		}
		current = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, filepath.FromSlash(value))
}

func PackageDir(root string, manifest Manifest) string {
	return filepath.Dir(resolvePath(root, manifest.Package))
}
