package appcmd

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

const DefaultViewPath = "app/views"

func Find(dir string, cmdPath string) (string, error) {
	modulePath, err := ModuleName(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(cmdPath) != "" {
		return explicitCommandPath(dir, cmdPath, modulePath)
	}

	cmdDir := filepath.Join(dir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("application command not found; ./cmd does not exist")
		}
		return "", fmt.Errorf("read ./cmd: %w", err)
	}

	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			candidates = append(candidates, entry.Name())
		}
	}
	slices.Sort(candidates)

	for _, name := range candidates {
		candidate := filepath.Join("cmd", name)
		if isLazyAppCommandDir(filepath.Join(dir, candidate), dir, modulePath) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("application command not found; no GoLazy application commands under ./cmd")
}

func ModuleName(filename string) (string, error) {
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

func GoRunArgs(tags string, commandPath string, viewPath string) []string {
	return []string{
		"run",
		"-tags",
		tags,
		"-ldflags",
		ViewPathLDFlags(viewPath),
		goRunPath(commandPath),
	}
}

func GoBuildArgs(tags string, commandPath string, viewPath string, outputPath string) []string {
	return []string{
		"build",
		"-tags",
		tags,
		"-ldflags",
		ViewPathLDFlags(viewPath),
		"-o",
		outputPath,
		goRunPath(commandPath),
	}
}

func ViewPathLDFlags(viewPath string) string {
	viewPath = strings.TrimSpace(viewPath)
	if viewPath == "" {
		viewPath = DefaultViewPath
	}
	return "-X golazy.dev/lazyviews.ViewsPath=" + viewPath
}

func explicitCommandPath(dir string, cmdPath string, modulePath string) (string, error) {
	cmdPath = filepath.Clean(strings.TrimSpace(cmdPath))
	var path string
	if filepath.IsAbs(cmdPath) {
		path = cmdPath
	} else {
		path = filepath.Join(dir, cmdPath)
	}
	if !isLazyAppCommandDir(path, dir, modulePath) {
		return "", fmt.Errorf("GoLazy application command not found at %q", cmdPath)
	}
	return cmdPath, nil
}

func isLazyAppCommandDir(path string, moduleRoot string, modulePath string) bool {
	if !isMainPackageDir(path) {
		return false
	}
	return importsLazyApp(path, moduleRoot, modulePath, map[string]bool{})
}

func isMainPackageDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(files, filepath.Join(path, name), nil, parser.PackageClauseOnly)
		if err == nil && file.Name.Name == "main" {
			return true
		}
	}
	return false
}

func importsLazyApp(dir string, moduleRoot string, modulePath string, seen map[string]bool) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	if seen[abs] {
		return false
	}
	seen[abs] = true

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(files, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			continue
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				continue
			}
			if importPath == "golazy.dev/lazyapp" {
				return true
			}
			localDir, ok := localImportDir(importPath, moduleRoot, modulePath)
			if ok && importsLazyApp(localDir, moduleRoot, modulePath, seen) {
				return true
			}
		}
	}
	return false
}

func localImportDir(importPath string, moduleRoot string, modulePath string) (string, bool) {
	if importPath == modulePath {
		return moduleRoot, true
	}
	prefix := modulePath + "/"
	if strings.HasPrefix(importPath, prefix) {
		return filepath.Join(moduleRoot, filepath.FromSlash(strings.TrimPrefix(importPath, prefix))), true
	}
	return "", false
}

func goRunPath(commandPath string) string {
	commandPath = filepath.ToSlash(filepath.Clean(commandPath))
	if commandPath == "." || filepath.IsAbs(commandPath) {
		return commandPath
	}
	return "./" + commandPath
}
