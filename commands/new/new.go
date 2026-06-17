package newcommand

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/golazy/lazy/commands"
	"golang.org/x/mod/modfile"
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
	if err := replaceSecureCookieKey(destination); err != nil {
		return err
	}

	commandEnv, goArgs, cleanupWorkfile, err := localValidation(destination, c.SourceDir != "")
	if err != nil {
		return err
	}
	defer cleanupWorkfile()

	fmt.Fprintln(c.Stdout, "* Validating")
	tidyArgs := append([]string{"mod", "tidy"}, goArgs...)
	if err := runner("go", tidyArgs, commands.Options{
		Dir:     destination,
		Env:     commandEnv,
		Capture: true,
	}); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	testArgs := append([]string{"test"}, goArgs...)
	testArgs = append(testArgs, "./...")
	if err := runner("go", testArgs, commands.Options{
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
		if relative == "node_modules" || strings.Contains(relative, string(filepath.Separator)+"node_modules"+string(filepath.Separator)) {
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

func localValidation(destination string, sourceDir bool) ([]string, []string, func(), error) {
	if sourceDir {
		env, args, cleanup, err := localModfileValidation(destination)
		if err != nil || len(args) != 0 {
			return env, args, cleanup, err
		}
	}
	env, cleanup, err := localWorkspaceEnv(destination)
	return env, nil, cleanup, err
}

func localModfileValidation(destination string) ([]string, []string, func(), error) {
	workspaceFile, workspaceRoot, found := findWorkspace(destination)
	if !found {
		return nil, nil, func() {}, nil
	}

	data, err := os.ReadFile(workspaceFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read %s: %w", workspaceFile, err)
	}
	workFile, err := modfile.ParseWork(workspaceFile, data, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse %s: %w", workspaceFile, err)
	}
	if len(workFile.Replace) == 0 {
		return nil, nil, func() {}, nil
	}

	modPath := filepath.Join(destination, "go.mod")
	modData, err := os.ReadFile(modPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read %s: %w", modPath, err)
	}
	modFile, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse %s: %w", modPath, err)
	}

	added := false
	for _, replacement := range workFile.Replace {
		newPath := replacement.New.Path
		if replacement.New.Version == "" && newPath != "" && !filepath.IsAbs(newPath) {
			newPath = filepath.Join(workspaceRoot, filepath.FromSlash(newPath))
		}
		if err := modFile.AddReplace(replacement.Old.Path, replacement.Old.Version, newPath, replacement.New.Version); err != nil {
			return nil, nil, nil, fmt.Errorf("add temporary replace for %s: %w", replacement.Old.Path, err)
		}
		added = true
	}
	if !added {
		return nil, nil, func() {}, nil
	}

	tempMod := filepath.Join(destination, ".lazy-go.mod")
	formatted, err := modFile.Format()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("format temporary go.mod: %w", err)
	}
	if err := os.WriteFile(tempMod, formatted, 0o644); err != nil {
		return nil, nil, nil, fmt.Errorf("write %s: %w", tempMod, err)
	}

	cleanup := func() {
		_ = os.Remove(tempMod)
		_ = os.Remove(filepath.Join(destination, ".lazy-go.sum"))
	}
	env := []string{
		"GOWORK=off",
		"GOPRIVATE=golazy.dev",
		"GONOSUMDB=golazy.dev",
	}
	return env, []string{"-modfile=.lazy-go.mod"}, cleanup, nil
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

func replaceSecureCookieKey(root string) error {
	key, err := randomHexKey(8)
	if err != nil {
		return err
	}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		if _, err := replaceLazySessionConfigKey(path, key); err != nil {
			return err
		}
		if _, err := replaceGoStringConst(path, "secureCookieKey", key); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("replace secure cookie key: %w", err)
	}
	return nil
}

func randomHexKey(size int) (string, error) {
	key := make([]byte, size)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate secure cookie key: %w", err)
	}
	return hex.EncodeToString(key), nil
}

func replaceLazySessionConfigKey(path, value string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, data, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}

	var literal *ast.BasicLit
	ast.Inspect(parsed, func(node ast.Node) bool {
		if literal != nil {
			return false
		}
		composite, ok := node.(*ast.CompositeLit)
		if !ok || !isLazySessionConfigType(composite.Type) {
			return true
		}
		for _, element := range composite.Elts {
			keyValue, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			ident, ok := keyValue.Key.(*ast.Ident)
			if !ok || ident.Name != "Key" {
				continue
			}
			if lit, ok := keyValue.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				literal = lit
				return false
			}
		}
		return true
	})
	if literal == nil {
		return false, nil
	}

	if err := replaceStringLiteral(path, data, fileSet, literal, value); err != nil {
		return false, err
	}
	return true, nil
}

func isLazySessionConfigType(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Config" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "lazysession"
}

func replaceGoStringConst(path, name, value string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, data, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}

	var literal *ast.BasicLit
	ast.Inspect(parsed, func(node ast.Node) bool {
		if literal != nil {
			return false
		}
		valueSpec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, ident := range valueSpec.Names {
			if ident.Name != name || i >= len(valueSpec.Values) {
				continue
			}
			if lit, ok := valueSpec.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				literal = lit
				return false
			}
		}
		return true
	})
	if literal == nil {
		return false, nil
	}

	if err := replaceStringLiteral(path, data, fileSet, literal, value); err != nil {
		return false, err
	}
	return true, nil
}

func replaceStringLiteral(path string, data []byte, fileSet *token.FileSet, literal *ast.BasicLit, value string) error {
	start := fileSet.Position(literal.Pos()).Offset
	end := fileSet.Position(literal.End()).Offset
	updated := append([]byte(nil), data[:start]...)
	updated = append(updated, []byte(strconv.Quote(value))...)
	updated = append(updated, data[end:]...)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, updated, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
