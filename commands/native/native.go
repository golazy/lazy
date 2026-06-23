package native

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/appcmd"
	jscommand "golazy.dev/lazy/commands/js"
)

const helperRepo = "https://github.com/golazy/native"

type Command struct {
	Dir      string
	CmdPath  string
	ViewPath string
	Out      string
	Target   string
	Title    string
	Width    int
	Height   int
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer

	Runner       commands.Runner
	OutputRunner commands.OutputRunner
	UserCacheDir func() (string, error)
	TempDir      func(dir string, pattern string) (string, error)
	RemoveAll    func(path string) error
	MkdirAll     func(path string, perm os.FileMode) error
	Stat         func(path string) (os.FileInfo, error)
	ReadDir      func(path string) ([]os.DirEntry, error)
	Executable   func() (string, error)
	Listen       func(network string, address string) (net.Listener, error)
	GOOS         string
	GOARCH       string
}

func (c Command) ExecuteDev() (int, error) {
	if err := c.ensureSupportedHost(); err != nil {
		return 1, err
	}
	dir, err := c.workingDir()
	if err != nil {
		return 1, err
	}
	helper, err := c.helperBinary()
	if err != nil {
		return 1, err
	}
	addr, err := c.freeAddr()
	if err != nil {
		return 1, err
	}
	lazyBinary, err := c.executable()
	if err != nil {
		return 1, fmt.Errorf("find lazy executable: %w", err)
	}

	args := []string{
		"--dir", dir,
		"--addr", addr,
		"--env", "ADDR=" + addr,
		"--env", "LAZY_TMUX=1",
		"--env", "NO_VERSION_CHECK=true",
	}
	if c.Title != "" {
		args = append(args, "--title", c.Title)
	}
	if c.Width > 0 {
		args = append(args, "--width", strconv.Itoa(c.Width))
	}
	if c.Height > 0 {
		args = append(args, "--height", strconv.Itoa(c.Height))
	}
	args = append(args, "--", lazyBinary, "--skip-version-check")
	if c.CmdPath != "" {
		args = append(args, "--cmdpath", filepath.ToSlash(c.CmdPath))
	}
	if c.ViewPath != "" && c.ViewPath != appcmd.DefaultViewPath {
		args = append(args, "--viewpath", filepath.ToSlash(c.ViewPath))
	}

	if err := c.runner()(helper, args, commands.Options{
		Dir:    dir,
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}); err != nil {
		var processExit *commands.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, fmt.Errorf("run native helper: %w", err)
	}
	return 0, nil
}

func (c Command) ExecuteBuild() (int, error) {
	if err := c.ensureSupportedHost(); err != nil {
		return 1, err
	}
	dir, err := c.workingDir()
	if err != nil {
		return 1, err
	}
	target, err := c.normalizeTarget()
	if err != nil {
		return 1, err
	}
	helper, err := c.helperBinary()
	if err != nil {
		return 1, err
	}
	candidate, err := appcmd.Find(dir, c.CmdPath)
	if err != nil {
		return 1, err
	}
	if code, err := c.prepareGeneratedAssets(dir); code != 0 || err != nil {
		return code, err
	}

	workDir, err := c.tempDir("", "lazy-native-app-*")
	if err != nil {
		return 1, fmt.Errorf("create temporary native build directory: %w", err)
	}
	defer c.removeAll()(workDir)

	appBinary := filepath.Join(workDir, "app"+exeSuffix(c.goos()))
	buildCommand, buildArgs, buildEnv := commands.MiseExecRunnerCommand(c.Runner, "go", appcmd.GoBuildArgs("", filepath.ToSlash(candidate), appBinary))
	if err := c.runner()(buildCommand, buildArgs, commands.Options{
		Dir:    dir,
		Env:    buildEnv,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}); err != nil {
		var processExit *commands.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, fmt.Errorf("build application binary: %w", err)
	}

	out := c.Out
	if strings.TrimSpace(out) == "" {
		out = filepath.Join("dist", "native", target)
	}
	args := []string{
		"build",
		"--dir", dir,
		"--target", target,
		"--app-binary", appBinary,
		"--out", out,
	}
	if err := c.runner()(helper, args, commands.Options{
		Dir:    dir,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}); err != nil {
		var processExit *commands.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, fmt.Errorf("run native helper: %w", err)
	}
	return 0, nil
}

func (c Command) prepareGeneratedAssets(dir string) (int, error) {
	_, err := c.stat()(filepath.Join(dir, "js.toml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 1, fmt.Errorf("inspect js.toml: %w", err)
	}
	code, err := (jscommand.Command{
		Dir:    dir,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
		Runner: c.runner(),
	}).Execute()
	if err != nil || code != 0 {
		return code, err
	}
	return 0, nil
}

func (c Command) helperBinary() (string, error) {
	cacheRoot, err := c.cacheRoot()
	if err != nil {
		return "", err
	}

	commit, latestErr := c.latestCommit()
	if latestErr == nil && commit != "" {
		binary := filepath.Join(cacheRoot, commit, helperName(c.goos()))
		if _, err := c.stat()(binary); err == nil {
			return binary, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect native helper: %w", err)
		}
		if err := c.installHelper(commit, binary); err == nil {
			return binary, nil
		}
	}

	cached, cacheErr := c.latestCachedHelper(cacheRoot)
	if cacheErr != nil {
		return "", cacheErr
	}
	if cached != "" {
		return cached, nil
	}
	if latestErr != nil {
		return "", fmt.Errorf("resolve github.com/golazy/native HEAD: %w", latestErr)
	}
	return "", fmt.Errorf("native helper is not available")
}

func (c Command) latestCommit() (string, error) {
	output, err := c.outputRunner()("git", []string{"ls-remote", helperRepo, "HEAD"}, commands.Options{
		Stderr: io.Discard,
	})
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return "", fmt.Errorf("git ls-remote returned no commit")
	}
	return fields[0], nil
}

func (c Command) installHelper(commit string, binary string) error {
	if err := c.mkdirAll()(filepath.Dir(binary), 0o755); err != nil {
		return fmt.Errorf("create native helper cache: %w", err)
	}
	tmp, err := c.tempDir("", "lazy-native-helper-*")
	if err != nil {
		return fmt.Errorf("create temporary native helper directory: %w", err)
	}
	defer c.removeAll()(tmp)

	src := filepath.Join(tmp, "native")
	if err := c.runner()("git", []string{"clone", "--depth", "1", helperRepo, src}, commands.Options{
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}); err != nil {
		return fmt.Errorf("clone github.com/golazy/native: %w", err)
	}
	if commit != "" {
		if err := c.runner()("git", []string{"checkout", commit}, commands.Options{
			Dir:    src,
			Stdout: c.Stdout,
			Stderr: c.Stderr,
		}); err != nil {
			return fmt.Errorf("checkout native helper %s: %w", commit, err)
		}
	}
	if err := c.runner()("go", []string{"build", "-o", binary, "."}, commands.Options{
		Dir:    src,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}); err != nil {
		return fmt.Errorf("build native helper: %w", err)
	}
	return nil
}

func (c Command) latestCachedHelper(cacheRoot string) (string, error) {
	entries, err := c.readDir()(cacheRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read native helper cache: %w", err)
	}
	type candidate struct {
		path string
		time int64
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		binary := filepath.Join(cacheRoot, entry.Name(), helperName(c.goos()))
		info, err := c.stat()(binary)
		if err != nil || info.IsDir() {
			continue
		}
		candidates = append(candidates, candidate{
			path: binary,
			time: info.ModTime().UnixNano(),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].time > candidates[j].time
	})
	if len(candidates) == 0 {
		return "", nil
	}
	return candidates[0].path, nil
}

func (c Command) cacheRoot() (string, error) {
	dir, err := c.userCacheDir()()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	return filepath.Join(dir, "golazy", "native", "builds"), nil
}

func (c Command) freeAddr() (string, error) {
	listener, err := c.listen()("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("find free native development port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().String(), nil
}

func (c Command) normalizeTarget() (string, error) {
	target := strings.ToLower(strings.TrimSpace(c.Target))
	goos := c.goos()
	goarch := c.goarch()
	current := goos + "-" + goarch
	switch target {
	case "", "current", current:
		return current, nil
	case "macos", "darwin":
		if goos == "darwin" {
			return current, nil
		}
	case "linux":
		if goos == "linux" {
			return current, nil
		}
	default:
		if target == "macos-"+goarch {
			target = "darwin-" + goarch
		}
		if target == current {
			return current, nil
		}
	}
	return "", fmt.Errorf("native build target %q is not the current platform %s", c.Target, current)
}

func (c Command) ensureSupportedHost() error {
	switch c.goos() {
	case "darwin", "linux":
		return nil
	default:
		return fmt.Errorf("lazy native currently supports macOS and Linux hosts; current host is %s", c.goos())
	}
}

func (c Command) workingDir() (string, error) {
	if c.Dir != "" {
		return c.Dir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return dir, nil
}

func (c Command) runner() commands.Runner {
	if c.Runner != nil {
		return c.Runner
	}
	return commands.Exec
}

func (c Command) outputRunner() commands.OutputRunner {
	if c.OutputRunner != nil {
		return c.OutputRunner
	}
	return commands.ExecOutput
}

func (c Command) userCacheDir() func() (string, error) {
	if c.UserCacheDir != nil {
		return c.UserCacheDir
	}
	return os.UserCacheDir
}

func (c Command) tempDir(dir string, pattern string) (string, error) {
	if c.TempDir != nil {
		return c.TempDir(dir, pattern)
	}
	return os.MkdirTemp(dir, pattern)
}

func (c Command) removeAll() func(string) error {
	if c.RemoveAll != nil {
		return c.RemoveAll
	}
	return os.RemoveAll
}

func (c Command) mkdirAll() func(string, os.FileMode) error {
	if c.MkdirAll != nil {
		return c.MkdirAll
	}
	return os.MkdirAll
}

func (c Command) stat() func(string) (os.FileInfo, error) {
	if c.Stat != nil {
		return c.Stat
	}
	return os.Stat
}

func (c Command) readDir() func(string) ([]os.DirEntry, error) {
	if c.ReadDir != nil {
		return c.ReadDir
	}
	return os.ReadDir
}

func (c Command) executable() (string, error) {
	if c.Executable != nil {
		return c.Executable()
	}
	return os.Executable()
}

func (c Command) listen() func(string, string) (net.Listener, error) {
	if c.Listen != nil {
		return c.Listen
	}
	return net.Listen
}

func (c Command) goos() string {
	if c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c Command) goarch() string {
	if c.GOARCH != "" {
		return c.GOARCH
	}
	return runtime.GOARCH
}

func helperName(goos string) string {
	if goos == "windows" {
		return "golazy-native.exe"
	}
	return "golazy-native"
}

func exeSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}
