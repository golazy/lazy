package scaffoldservice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golazy.dev/lazy/services/execservice"
)

const sampleRepository = "https://github.com/golazy/sample_app"
const defaultLatestVersionURL = "https://golazy.dev/lazy.version"
const latestVersionTimeout = time.Second

const SampleRepository = sampleRepository
const DefaultLatestVersionURL = defaultLatestVersionURL

type LatestVersionFetcher func(ctx context.Context, url string) (string, error)

type Command struct {
	Version              string
	CurrentVersion       string
	SourceDir            string
	Dir                  string
	GoWork               string
	SkipUpdateCheck      bool
	LatestVersionURL     string
	LatestVersionFetcher LatestVersionFetcher
	Stdout               io.Writer
	Stderr               io.Writer
	Runner               execservice.Runner
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

	templateVersion := strings.TrimSpace(c.Version)
	if c.SourceDir == "" && !semver.IsValid(templateVersion) {
		return fmt.Errorf("version %q is not a valid semantic version", c.Version)
	}
	if c.SourceDir == "" && !c.SkipUpdateCheck {
		if err := c.checkLatestVersion(); err != nil {
			return err
		}
	}

	runner := c.Runner
	if runner == nil {
		runner = execservice.Exec
	}
	miseCommand := "mise"
	var miseEnv []string
	if c.Runner == nil {
		miseCommand, miseEnv = execservice.ResolveMiseCommand()
	}

	fmt.Fprintln(c.Stdout, "* Initializing the core app")
	if c.SourceDir != "" {
		if err := copyTemplateDirectory(resolveSourceDir(dir, c.SourceDir), destination); err != nil {
			return err
		}
	} else {
		if err := runner("git", []string{
			"clone",
			"--branch", templateVersion,
			"--depth", "1",
			"--single-branch",
			sampleRepository,
			destination,
		}, execservice.Options{Dir: dir, Capture: true}); err != nil {
			return fmt.Errorf("clone sample app at %s: %w", templateVersion, err)
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
	if err := replaceTemplateName(destination, modulePath, appName); err != nil {
		return err
	}
	if err := renameTemplateCommand(destination, appName); err != nil {
		return err
	}
	if err := replaceSecureCookieKey(destination); err != nil {
		return err
	}

	commandEnv, workspaceValidation, cleanupWorkfile, err := localValidation(destination, c.GoWork)
	if err != nil {
		return err
	}
	defer cleanupWorkfile()

	fmt.Fprintln(c.Stdout, "* Preparing the mise development environment")
	if err := runner(miseCommand, []string{"trust", "--yes", "mise.toml"}, execservice.Options{
		Dir:    destination,
		Env:    miseEnv,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}); err != nil {
		return fmt.Errorf("mise trust: %w", err)
	}
	if err := runner(miseCommand, []string{"install", "--yes"}, execservice.Options{
		Dir:    destination,
		Env:    miseEnv,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}); err != nil {
		return fmt.Errorf("mise install: %w", err)
	}

	fmt.Fprintln(c.Stdout, "* Validating")
	syncArgs := []string{"mod", "tidy"}
	if workspaceValidation {
		syncArgs = []string{"work", "sync"}
	}
	if err := runner("go", syncArgs, execservice.Options{
		Dir:     destination,
		Env:     commandEnv,
		Capture: true,
	}); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(append([]string{"go"}, syncArgs...), " "), err)
	}
	testArgs := []string{"test"}
	testArgs = append(testArgs, "./...")
	if err := runner("go", testArgs, execservice.Options{
		Dir:     destination,
		Env:     commandEnv,
		Capture: true,
	}); err != nil {
		return fmt.Errorf("go test ./...: %w", err)
	}

	cleanupWorkfile()
	cleanupWorkfile = func() {}

	fmt.Fprintln(c.Stdout, "* Saving the initial Git commit")
	if err := initializeGitRepository(runner, destination); err != nil {
		return err
	}

	cleanup = false
	fmt.Fprintln(c.Stdout, "Next steps:")
	fmt.Fprintf(c.Stdout, "  cd %s\n", appName)
	fmt.Fprintln(c.Stdout, "  lazy")
	return nil
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func ExecutableName(name string) string {
	return executableName(name)
}

func resolveMiseCommand() (string, []string) {
	return execservice.ResolveMiseCommand()
}

func ResolveMiseCommand() (string, []string) {
	return resolveMiseCommand()
}

func initializeGitRepository(runner execservice.Runner, destination string) error {
	steps := []struct {
		args []string
		name string
	}{
		{args: []string{"init"}, name: "git init"},
		{args: []string{"add", "."}, name: "git add"},
		{
			args: []string{
				"-c", "user.name=GoLazy",
				"-c", "user.email=noreply@golazy.dev",
				"commit", "-m", "Initial GoLazy application",
			},
			name: "git commit",
		},
	}

	for _, step := range steps {
		if err := runner("git", step.args, execservice.Options{Dir: destination, Capture: true}); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	return nil
}

func (c Command) checkLatestVersion() error {
	current := strings.TrimSpace(c.CurrentVersion)
	if current == "" {
		current = strings.TrimSpace(c.Version)
	}
	if !semver.IsValid(current) {
		return nil
	}

	url := strings.TrimSpace(c.LatestVersionURL)
	if url == "" {
		url = defaultLatestVersionURL
	}
	fetcher := c.LatestVersionFetcher
	if fetcher == nil {
		fetcher = fetchLatestVersion
	}

	ctx, cancel := context.WithTimeout(context.Background(), latestVersionTimeout)
	defer cancel()
	latest, err := fetcher(ctx, url)
	if err != nil {
		return nil
	}
	latest = strings.TrimSpace(latest)
	if !semver.IsValid(latest) || semver.Compare(latest, current) <= 0 {
		return nil
	}

	return fmt.Errorf("lazy %s is available; this binary is %s. lazy new uses the CLI version to choose the app template. Update lazy and rerun, or pass --skip-update-check to create from %s", latest, current, strings.TrimSpace(c.Version))
}

func fetchLatestVersion(ctx context.Context, url string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("latest version endpoint returned %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 128))
	if err != nil {
		return "", err
	}
	return string(data), nil
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

func localValidation(destination string, goWork string) ([]string, bool, func(), error) {
	env, cleanup, err := localWorkspaceEnv(destination, goWork)
	return env, len(env) != 0, cleanup, err
}

func localWorkspaceEnv(destination string, goWork string) ([]string, func(), error) {
	workspaceFile, workspaceRoot, found := findWorkspace(destination, goWork)
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

func findWorkspace(start string, goWork string) (string, string, bool) {
	if current := strings.TrimSpace(goWork); current != "" {
		if strings.EqualFold(current, "off") {
			return "", "", false
		}
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

func replaceTemplateName(root, modulePath, appName string) error {
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

	replacements := []struct {
		old []byte
		new []byte
	}{
		{old: []byte("sample_app"), new: []byte(modulePath)},
		{old: []byte("cmd/app"), new: []byte("cmd/" + appName)},
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if !utf8.Valid(data) {
			continue
		}

		updated := data
		for _, replacement := range replacements {
			updated = bytes.ReplaceAll(updated, replacement.old, replacement.new)
		}
		if bytes.Equal(updated, data) {
			continue
		}

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

func renameTemplateCommand(root, appName string) error {
	source := filepath.Join(root, "cmd", "app")
	target := filepath.Join(root, "cmd", appName)
	if source == target {
		return nil
	}

	info, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect command template directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("command template path %q is not a directory", filepath.Join("cmd", "app"))
	}

	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("command directory %q already exists", filepath.Join("cmd", appName))
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect command directory %q: %w", filepath.Join("cmd", appName), err)
	}

	if err := os.Rename(source, target); err != nil {
		return fmt.Errorf("rename command directory %q to %q: %w", filepath.Join("cmd", "app"), filepath.Join("cmd", appName), err)
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

func ReplaceSecureCookieKey(root string) error {
	return replaceSecureCookieKey(root)
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
