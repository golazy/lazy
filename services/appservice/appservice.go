package appservice

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
const DefaultPublicPath = "app/public"

const lazyappViewsPathSymbol = "golazy.dev/lazyapp.ViewsPath"
const lazyappPublicPathSymbol = "golazy.dev/lazyapp.PublicPath"

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

func GoRunArgs(tags string, commandPath string, buildFlags ...string) []string {
	args := []string{"run"}
	if strings.TrimSpace(tags) != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, buildFlags...)
	return append(args, goRunPath(commandPath))
}

func GoBuildArgs(tags string, commandPath string, outputPath string, buildFlags ...string) []string {
	args := []string{"build"}
	if strings.TrimSpace(tags) != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, buildFlags...)
	return append(args, "-o", outputPath, goRunPath(commandPath))
}

type LazyDevPaths struct {
	Views  string
	Public string
}

func LazyDevBuildFlags(root string, viewPath string, publicPath string) ([]string, error) {
	paths, err := ResolveLazyDevPaths(root, viewPath, publicPath)
	if err != nil {
		return nil, err
	}
	return []string{"-ldflags", LazyDevLDFlags(paths)}, nil
}

func LazyDevLDFlags(paths LazyDevPaths) string {
	return strings.Join([]string{
		"-X", ldflagAssignment(lazyappViewsPathSymbol, paths.Views),
		"-X", ldflagAssignment(lazyappPublicPathSymbol, paths.Public),
	}, " ")
}

func ldflagAssignment(symbol string, value string) string {
	field := symbol + "=" + value
	if strings.ContainsAny(field, " \t\r\n\"'") {
		return strconv.Quote(field)
	}
	return field
}

func ResolveLazyDevPaths(root string, viewPath string, publicPath string) (LazyDevPaths, error) {
	views, err := ResolveViewPath(root, viewPath)
	if err != nil {
		return LazyDevPaths{}, err
	}
	public, err := ResolvePublicPath(root, publicPath)
	if err != nil {
		return LazyDevPaths{}, err
	}
	return LazyDevPaths{Views: views, Public: public}, nil
}

func ResolveViewPath(root string, viewPath string) (string, error) {
	viewPath = strings.TrimSpace(viewPath)
	if viewPath == "" {
		viewPath = DefaultViewPath
	}
	if filepath.IsAbs(viewPath) {
		return validateViewPath(viewPath)
	}
	return validateViewPath(filepath.Join(root, viewPath))
}

func ResolvePublicPath(root string, publicPath string) (string, error) {
	publicPath = strings.TrimSpace(publicPath)
	if publicPath == "" {
		publicPath = DefaultPublicPath
	}
	if filepath.IsAbs(publicPath) {
		return validatePublicPath(publicPath)
	}
	return validatePublicPath(filepath.Join(root, publicPath))
}

func validateViewPath(viewPath string) (string, error) {
	abs, err := filepath.Abs(viewPath)
	if err != nil {
		return "", fmt.Errorf("resolve views path %q: %w", viewPath, err)
	}
	info, err := os.Stat(filepath.Join(abs, "layouts", "app.html.tpl"))
	if err != nil {
		return "", fmt.Errorf("views path %q does not contain layouts/app.html.tpl: %w", viewPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("views path %q contains a directory at layouts/app.html.tpl", viewPath)
	}
	return abs, nil
}

func validatePublicPath(publicPath string) (string, error) {
	abs, err := filepath.Abs(publicPath)
	if err != nil {
		return "", fmt.Errorf("resolve public path %q: %w", publicPath, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("public path %q is not readable: %w", publicPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("public path %q is not a directory", publicPath)
	}
	return abs, nil
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
