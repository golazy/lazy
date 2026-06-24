package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	jscommand "golazy.dev/lazy/commands/js"
	"golazy.dev/lazy/commands/run/devapp"
	"golazy.dev/lazy/commands/run/reloadproxy"
	"golazy.dev/lazy/commands/run/watcher"
	"golazy.dev/lazytui/progress"
)

const (
	defaultPollInterval   = 500 * time.Millisecond
	defaultDebounce       = 150 * time.Millisecond
	defaultStartupTimeout = 10 * time.Second
	stopTimeout           = 2 * time.Second
)

type javaScriptAssetMode int

const (
	javaScriptAssetNone javaScriptAssetMode = iota
	javaScriptAssetBundle
	javaScriptAssetFull
)

type generatedAssetResult struct {
	Output   string
	Err      error
	Duration time.Duration
}

type devRunner struct {
	root        string
	commandPath string
	viewPath    string
	listenAddr  string
	goWork      string
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer

	pollInterval   time.Duration
	debounce       time.Duration
	startupTimeout time.Duration
}

func (d *devRunner) run(ctx context.Context) (int, error) {
	if d.stdout == nil {
		d.stdout = io.Discard
	}
	if d.stderr == nil {
		d.stderr = io.Discard
	}
	if d.pollInterval <= 0 {
		d.pollInterval = defaultPollInterval
	}
	if d.debounce <= 0 {
		d.debounce = defaultDebounce
	}
	if d.startupTimeout <= 0 {
		d.startupTimeout = defaultStartupTimeout
	}
	if d.listenAddr == "" {
		d.listenAddr = publicListenAddr("", "")
	}

	tmpDir, err := os.MkdirTemp("", "lazy-run-*")
	if err != nil {
		return 1, fmt.Errorf("create temporary build directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var proxy *reloadproxy.Proxy
	if err := d.runProgress(progress.Tasks{
		progress.Task("Start reload proxy", func(_ io.Reader, _ io.Writer, _ io.Writer) error {
			next, err := reloadproxy.New(d.listenAddr)
			if err != nil {
				return err
			}
			next.UpdateStatus(reloadproxy.Status{
				State:       reloadproxy.StateQueued,
				Message:     "Waiting for the first build.",
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
			})
			if err := next.Start(); err != nil {
				return err
			}
			proxy = next
			return nil
		}),
	}); err != nil {
		return 1, err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()
		_ = proxy.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(d.stderr, "lazy: serving %s with hot reload\n", displayListenAddr(proxy.Addr()))

	fileWatcher := watcher.Watcher{
		Root:     d.root,
		Interval: d.pollInterval,
		Debounce: d.debounce,
	}
	changeCh := fileWatcher.Watch(ctx)

	app := devapp.Config{
		Root:           d.root,
		CommandPath:    d.commandPath,
		ViewPath:       d.viewPath,
		GoWork:         d.goWork,
		Stdin:          d.stdin,
		Stdout:         d.stdout,
		Stderr:         d.stderr,
		StartupTimeout: d.startupTimeout,
		StopTimeout:    stopTimeout,
	}
	var current *devapp.Process
	var appDone <-chan error
	buildNumber := 0
	rebuild := func(reason string, changed []string) {
		buildNumber++
		started := time.Now()
		proxy.UpdateStatus(reloadproxy.Status{
			State:       reloadproxy.StateBuilding,
			Message:     reason,
			CommandPath: d.commandPath,
			WatchedRoot: d.root,
			BuildCount:  buildNumber,
		})
		assetMode := javaScriptAssetGenerationMode(d.root, changed)
		var assets generatedAssetResult
		var result devapp.BuildResult
		var next *devapp.Process
		tasks := make(progress.Tasks, 0, 3)
		if assetMode != javaScriptAssetNone {
			tasks = append(tasks, progress.Task(javaScriptAssetTaskName(assetMode), func(_ io.Reader, _ io.Writer, stderr io.Writer) error {
				assets = d.generateAssetsForMode(assetMode)
				if assets.Output != "" {
					printOutput(stderr, assets.Output)
				}
				return assets.Err
			}))
		}
		tasks = append(tasks, progress.Task("Build application", func(_ io.Reader, _ io.Writer, stderr io.Writer) error {
			result = app.Build(ctx, tmpDir, buildNumber)
			if result.Output != "" {
				printOutput(stderr, result.Output)
			}
			return result.Err
		}))
		tasks = append(tasks, progress.UITask("Start application", func(ui *progress.UI) error {
			return ui.Takeover(func(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
				startApp := app
				startApp.Stdin = stdin
				startApp.Stdout = stdout
				startApp.Stderr = stderr
				var err error
				next, err = startApp.Start(ctx, result.Binary)
				return err
			})
		}))

		err := d.runProgress(tasks)
		output := combineOutput(assets.Output, result.Output)
		duration := time.Since(started)
		if err != nil {
			if assets.Err != nil {
				proxy.UpdateStatus(reloadproxy.Status{
					State:       reloadproxy.StateBuildFailed,
					Message:     fmt.Sprintf("JavaScript generation failed after %s.", assets.Duration.Round(time.Millisecond)),
					CommandPath: d.commandPath,
					WatchedRoot: d.root,
					BuildCount:  buildNumber,
					Duration:    assets.Duration,
					Output:      assets.Output,
				})
				return
			}
			if result.Err != nil {
				proxy.UpdateStatus(reloadproxy.Status{
					State:       reloadproxy.StateBuildFailed,
					Message:     fmt.Sprintf("Build failed after %s.", duration.Round(time.Millisecond)),
					CommandPath: d.commandPath,
					WatchedRoot: d.root,
					BuildCount:  buildNumber,
					Duration:    duration,
					Output:      output,
				})
				return
			}
			proxy.UpdateStatus(reloadproxy.Status{
				State:       reloadproxy.StateRunFailed,
				Message:     err.Error(),
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
				BuildCount:  buildNumber,
				Duration:    duration,
				Output:      output,
			})
			return
		}

		old := current
		current = next
		appDone = next.Done()
		proxy.SetTarget("http://" + next.Addr())
		proxy.UpdateStatus(reloadproxy.Status{
			State:       reloadproxy.StateRunning,
			Message:     "Application is running.",
			CommandPath: d.commandPath,
			WatchedRoot: d.root,
			BuildCount:  buildNumber,
			Duration:    duration,
			StartedAt:   time.Now(),
			Changed:     changed,
		})
		if old != nil {
			old.Stop()
		}
		if buildNumber > 1 {
			proxy.BroadcastReload()
		}
		fmt.Fprintf(d.stderr, "lazy: application running after %s\n", duration.Round(time.Millisecond))
	}

	rebuild("Starting initial build.", nil)

	for {
		select {
		case <-ctx.Done():
			proxy.ClearTarget()
			proxy.UpdateStatus(reloadproxy.Status{
				State:       reloadproxy.StateStopped,
				Message:     "lazy is shutting down.",
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
				BuildCount:  buildNumber,
			})
			if current != nil {
				current.Stop()
			}
			return 0, nil
		case changed, ok := <-changeCh:
			if !ok {
				changeCh = nil
				continue
			}
			changed = drainChanges(changeCh, changed)
			if onlyGeneratedJavaScriptOutputs(changed) {
				continue
			}
			rebuild(fmt.Sprintf("Rebuilding after %d changed file(s).", len(changed)), changed)
		case err := <-appDone:
			appDone = nil
			current = nil
			proxy.ClearTarget()
			message := "Application exited."
			if err != nil {
				message = fmt.Sprintf("Application exited: %v", err)
			}
			proxy.UpdateStatus(reloadproxy.Status{
				State:       reloadproxy.StateRunFailed,
				Message:     message,
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
				BuildCount:  buildNumber,
			})
			fmt.Fprintf(d.stderr, "lazy: %s\n", message)
			if shouldExitAfterApplicationDone(ctx, err) {
				return 0, nil
			}
		}
	}
}

func shouldExitAfterApplicationDone(ctx context.Context, err error) bool {
	return ctx.Err() != nil || err == nil || devapp.Interrupted(err)
}

func (d *devRunner) runProgress(tasks progress.Tasks) error {
	return progress.Run(tasks, d.stdin, d.stdout, d.stderr)
}

func (d *devRunner) generateAssets(changed []string) generatedAssetResult {
	return d.generateAssetsForMode(javaScriptAssetGenerationMode(d.root, changed))
}

func (d *devRunner) generateAssetsForMode(mode javaScriptAssetMode) generatedAssetResult {
	if mode == javaScriptAssetNone {
		return generatedAssetResult{}
	}

	started := time.Now()
	switch mode {
	case javaScriptAssetFull:
		var output bytes.Buffer
		code, err := (jscommand.Command{
			Dir:    d.root,
			Stdout: &output,
			Stderr: &output,
		}).Execute()
		if err == nil && code != 0 {
			err = fmt.Errorf("lazy js failed with exit code %d", code)
		}
		return generatedAssetResult{
			Output:   output.String(),
			Err:      err,
			Duration: time.Since(started),
		}
	case javaScriptAssetBundle:
		return bundleJavaScriptAssets(d.root, started)
	default:
		return generatedAssetResult{}
	}
}

func javaScriptAssetTaskName(mode javaScriptAssetMode) string {
	switch mode {
	case javaScriptAssetFull:
		return "Generate JavaScript assets"
	case javaScriptAssetBundle:
		return "Bundle JavaScript assets"
	default:
		return "Generate JavaScript assets"
	}
}

func bundleJavaScriptAssets(root string, started time.Time) generatedAssetResult {
	const output = "* Bundling JavaScript\n"

	manifest, err := jscommand.LoadManifest(root)
	if err != nil {
		return generatedAssetResult{Output: output, Err: err, Duration: time.Since(started)}
	}
	packageDir := filepath.Dir(resolveRootPath(root, manifest.Package))
	if _, err := jscommand.Bundle(manifest, root, packageDir); err != nil {
		return generatedAssetResult{Output: output, Err: err, Duration: time.Since(started)}
	}
	return generatedAssetResult{Output: output, Duration: time.Since(started)}
}

func javaScriptAssetGenerationMode(root string, changed []string) javaScriptAssetMode {
	if !fileExists(filepath.Join(root, "js.toml")) {
		return javaScriptAssetNone
	}
	if changed == nil {
		return javaScriptAssetFull
	}

	mode := javaScriptAssetNone
	for _, path := range changed {
		if isJavaScriptPackageInput(path) {
			return javaScriptAssetFull
		}
		if isAppJavaScriptInput(path) {
			mode = javaScriptAssetBundle
		}
	}
	return mode
}

func isJavaScriptPackageInput(path string) bool {
	path = cleanWatchPath(path)
	switch path {
	case "js.toml", "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb":
		return true
	default:
		return false
	}
}

func isAppJavaScriptInput(path string) bool {
	return strings.HasPrefix(cleanWatchPath(path), "app/js/")
}

func onlyGeneratedJavaScriptOutputs(changed []string) bool {
	if len(changed) == 0 {
		return false
	}
	for _, path := range changed {
		if !isGeneratedJavaScriptOutput(path) {
			return false
		}
	}
	return true
}

func isGeneratedJavaScriptOutput(path string) bool {
	path = cleanWatchPath(path)
	return path == "app/public/assets/importmap.json" || strings.HasPrefix(path, "app/public/assets/lazyshaft/")
}

func cleanWatchPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func resolveRootPath(root, path string) string {
	candidate := filepath.FromSlash(path)
	if filepath.IsAbs(candidate) {
		return candidate
	}
	return filepath.Join(root, candidate)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func printOutput(w io.Writer, output string) {
	fmt.Fprint(w, output)
	if !strings.HasSuffix(output, "\n") {
		fmt.Fprintln(w)
	}
}

func combineOutput(parts ...string) string {
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		if builder.Len() > 0 && !strings.HasSuffix(builder.String(), "\n") {
			builder.WriteByte('\n')
		}
		builder.WriteString(part)
	}
	return builder.String()
}

func drainChanges(changeCh <-chan []string, changed []string) []string {
	seen := make(map[string]bool, len(changed))
	var combined []string
	for _, path := range changed {
		if !seen[path] {
			seen[path] = true
			combined = append(combined, path)
		}
	}
	for {
		select {
		case more, ok := <-changeCh:
			if !ok {
				return combined
			}
			for _, path := range more {
				if !seen[path] {
					seen[path] = true
					combined = append(combined, path)
				}
			}
		default:
			return combined
		}
	}
}

func publicListenAddr(addr string, port string) string {
	if addr != "" {
		return normalizeListenAddr(addr)
	}
	if port != "" {
		return normalizeListenAddr(port)
	}
	return ":3000"
}

func normalizeListenAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if _, err := strconv.ParseUint(addr, 10, 16); err == nil {
		return ":" + addr
	}
	return addr
}

func displayListenAddr(addr string) string {
	if strings.HasPrefix(addr, "[::]:") {
		return "http://localhost:" + strings.TrimPrefix(addr, "[::]:")
	}
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return "http://" + addr
}
