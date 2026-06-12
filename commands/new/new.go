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
	Version   string
	SourceDir string
	Dir       string
	Stdout    io.Writer
	Runner    commands.Runner
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
	if c.SourceDir != "" {
		if err := copyTemplateDirectory(resolveSourceDir(dir, c.SourceDir), destination); err != nil {
			return err
		}
	} else {
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
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(destination)
		}
	}()

	if c.SourceDir == "" {
		if err := os.RemoveAll(filepath.Join(destination, ".git")); err != nil {
			return fmt.Errorf("remove template Git metadata: %w", err)
		}
	}

	fmt.Fprintln(c.Stdout, "* Naming the app")
	if err := replaceTemplateName(destination, modulePath); err != nil {
		return err
	}

	commandEnv, cleanupWorkfile, err := localWorkspaceEnv(destination)
	if err != nil {
		return err
	}
	defer cleanupWorkfile()

	fmt.Fprintln(c.Stdout, "* Validating")
	if err := runner("go", []string{"mod", "tidy"}, commands.Options{
		Dir:     destination,
		Env:     commandEnv,
		Capture: true,
	}); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	if err := runner("go", []string{"test", "./..."}, commands.Options{
		Dir:     destination,
		Env:     commandEnv,
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

func resolveSourceDir(baseDir, sourceDir string) string {
	if filepath.IsAbs(sourceDir) {
		return sourceDir
	}
	return filepath.Join(baseDir, sourceDir)
}

func copyTemplateDirectory(source, destination string) error {
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("inspect source dir %q: %w", source, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source dir %q is not a directory", source)
	}

	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("resolve %s relative to source: %w", path, err)
		}
		if relative == "." {
			return os.MkdirAll(destination, info.Mode().Perm())
		}

		if relative == ".git" || strings.HasPrefix(relative, ".git"+string(filepath.Separator)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			dirInfo, dirErr := entry.Info()
			if dirErr != nil {
				return fmt.Errorf("stat %s: %w", path, dirErr)
			}
			return os.MkdirAll(target, dirInfo.Mode().Perm())
		}

		fileInfo, fileErr := entry.Info()
		if fileErr != nil {
			return fmt.Errorf("stat %s: %w", path, fileErr)
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		if writeErr := os.WriteFile(target, data, fileInfo.Mode().Perm()); writeErr != nil {
			return fmt.Errorf("write %s: %w", target, writeErr)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("copy source dir %q: %w", source, err)
	}

	return nil
}

func localWorkspaceEnv(destination string) ([]string, func(), error) {
	workspaceFile, workspaceRoot, found := findWorkspace(destination)
	if !found {
		return nil, func() {}, nil
	}

	relativeDestination, err := filepath.Rel(workspaceRoot, destination)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve destination relative to workspace: %w", err)
	}

	data, err := os.ReadFile(workspaceFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", workspaceFile, err)
	}

	tempFile, err := os.CreateTemp(workspaceRoot, ".lazy-go-work-*.work")
	if err != nil {
		return nil, nil, fmt.Errorf("create temporary go.work: %w", err)
	}
	defer tempFile.Close()

	var content strings.Builder
	content.Write(data)
	if len(data) != 0 && data[len(data)-1] != '\n' {
		content.WriteByte('\n')
	}
	usePath := filepath.ToSlash(destination)
	if !strings.HasPrefix(relativeDestination, "..") {
		usePath = "./" + filepath.ToSlash(relativeDestination)
	}
	content.WriteString("use (\n")
	content.WriteString("\t")
	content.WriteString(usePath)
	content.WriteString("\n)\n")

	if _, err := tempFile.WriteString(content.String()); err != nil {
		_ = os.Remove(tempFile.Name())
		return nil, nil, fmt.Errorf("write temporary go.work: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(tempFile.Name())
	}
	env := []string{
		"GOWORK=" + tempFile.Name(),
		"GOPRIVATE=golazy.dev",
		"GONOSUMDB=golazy.dev",
	}
	return env, cleanup, nil
}

func findWorkspace(start string) (string, string, bool) {
	if current := os.Getenv("GOWORK"); current != "" && current != "off" {
		if info, err := os.Stat(current); err == nil && !info.IsDir() {
			return current, filepath.Dir(current), true
		}
	}

	current := start
	for {
		candidate := filepath.Join(current, "go.work")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, current, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", "", false
		}
		current = parent
	}
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
