package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golazy.dev/lazy/commands/appcmd"
	"golazy.dev/lazy/commands/lazyconfig"
	appinit "golazy.dev/lazy/init"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazy/services/devserver"
	"golazy.dev/lazy/services/jsservice"
	"golazy.dev/lazy/services/lifecycleservice"
	"golazy.dev/lazy/services/tailwindservice"
	"golazy.dev/lazy/services/watchservice"
	"golazy.dev/lazytui/progress"
)

const (
	defaultPollInterval   = 500 * time.Millisecond
	defaultDebounce       = 150 * time.Millisecond
	defaultStartupTimeout = 10 * time.Second
	defaultListenAddr     = "127.0.0.1:3000"
	viewReloadTimeout     = 5 * time.Second
	stopTimeout           = 2 * time.Second
)

const lazyDevReloadViewsPath = "/views"

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

type viewReloadResult struct {
	Output   string
	Err      error
	Duration time.Duration
}

type devChangeAction int

const (
	devChangeRebuild devChangeAction = iota
	devChangeReloadBrowser
	devChangeReloadViews
)

type startupOutput struct {
	mu           sync.Mutex
	buffer       bytes.Buffer
	stdoutTarget io.Writer
	stderrTarget io.Writer
}

type startupOutputWriter struct {
	output *startupOutput
	stderr bool
}

type panelOutputWriter struct {
	store  *buildservice.Store
	stream string
}

type serviceOutputWriter struct {
	store   *buildservice.Store
	service string
	stream  string
}

func (w panelOutputWriter) Write(p []byte) (int, error) {
	if w.store != nil && len(p) > 0 {
		w.store.AddEvent(buildservice.Event{
			Type:   buildservice.EventOutput,
			Stream: w.stream,
			Output: string(p),
		})
	}
	return len(p), nil
}

func (w serviceOutputWriter) Write(p []byte) (int, error) {
	if w.store != nil && len(p) > 0 {
		w.store.AddEvent(buildservice.Event{
			Type:    buildservice.EventOutput,
			Stream:  w.stream,
			Service: w.service,
			Output:  string(p),
		})
	}
	return len(p), nil
}

func (o *startupOutput) Stdout() io.Writer {
	return startupOutputWriter{output: o}
}

func (o *startupOutput) Stderr() io.Writer {
	return startupOutputWriter{output: o, stderr: true}
}

func (o *startupOutput) Attach(stdout io.Writer, stderr io.Writer) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.buffer.Len() > 0 && stderr != nil {
		_, _ = stderr.Write(o.buffer.Bytes())
		o.buffer.Reset()
	}
	o.stdoutTarget = stdout
	o.stderrTarget = stderr
}

func (o *startupOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.String()
}

func (w startupOutputWriter) Write(p []byte) (int, error) {
	if w.output == nil {
		return len(p), nil
	}
	w.output.mu.Lock()
	defer w.output.mu.Unlock()
	target := w.output.stdoutTarget
	if w.stderr {
		target = w.output.stderrTarget
	}
	if target != nil {
		return target.Write(p)
	}
	return w.output.buffer.Write(p)
}

type devRunner struct {
	root          string
	commandPath   string
	viewPath      string
	publicPath    string
	listenAddr    string
	goWork        string
	serviceConfig lazyconfig.Config
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	forceKill     <-chan struct{}

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
		d.listenAddr = publicListenAddr(defaultListenAddr, 0)
	}

	tmpDir, err := os.MkdirTemp("", "lazy-run-*")
	if err != nil {
		return 1, fmt.Errorf("create temporary build directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	store := buildservice.NewStore(300)
	actions := buildservice.NewActions()
	panel := appinit.App(appinit.Config{Store: store, Actions: actions})

	var server *devserver.Server
	if err := d.runProgress(progress.Tasks{
		progress.Task("Start development panel", func(_ io.Reader, _ io.Writer, _ io.Writer) error {
			store.Update(buildservice.Snapshot{
				State:       buildservice.StateQueued,
				Message:     "Waiting for the first build.",
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
			})
			next, err := devserver.New(d.listenAddr, panel, store)
			if err != nil {
				return err
			}
			if err := next.Start(); err != nil {
				return err
			}
			server = next
			return nil
		}),
	}); err != nil {
		return 1, err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(d.stderr, "lazy: serving %s with development panel\n", displayListenAddr(server.Addr()))

	stdout := d.stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := d.stderr
	if stderr == nil {
		stderr = io.Discard
	}

	lifecycle, err := (lifecycleservice.Service{
		Dir:      d.root,
		Config:   d.serviceConfig,
		Stdin:    nil,
		Stdout:   stdout,
		Stderr:   stderr,
		Register: store.SetServices,
		Status: func(service string, state lifecycleservice.State, message string) {
			store.UpdateService(service, serviceState(state), message)
		},
		Output: func(service string, stream string) io.Writer {
			target := stdout
			if stream == "stderr" {
				target = stderr
			}
			return io.MultiWriter(target, serviceOutputWriter{store: store, service: service, stream: stream})
		},
	}).Start(ctx)
	if err != nil {
		return 1, err
	}
	defer stopLifecycleWithEscalation(lifecycle, d.forceKill)
	if lifecycle.Len() > 0 {
		store.Update(buildservice.Snapshot{
			State:       buildservice.StateStarting,
			Message:     "Starting development services.",
			CommandPath: d.commandPath,
			WatchedRoot: d.root,
		})
		if err := waitForLifecycle(ctx, lifecycle); err != nil {
			if ctx.Err() != nil {
				stopLifecycleWithEscalation(lifecycle, d.forceKill)
				return 0, nil
			}
			store.Update(buildservice.Snapshot{
				State:       buildservice.StateRunFailed,
				Message:     err.Error(),
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
			})
			return 1, err
		}
		store.AddEvent(buildservice.Event{Type: buildservice.EventManual, Message: "Development services are ready."})
	}

	fileWatcher := watchservice.Watcher{
		Root:     d.root,
		Interval: d.pollInterval,
		Debounce: d.debounce,
	}
	changeCh := fileWatcher.Watch(ctx)

	app := buildservice.Config{
		Root:           d.root,
		CommandPath:    d.commandPath,
		ViewPath:       d.viewPath,
		PublicPath:     d.publicPath,
		GoWork:         d.goWork,
		Stdin:          d.stdin,
		Stdout:         io.MultiWriter(stdout, panelOutputWriter{store: store, stream: "stdout"}),
		Stderr:         io.MultiWriter(stderr, panelOutputWriter{store: store, stream: "stderr"}),
		StartupTimeout: d.startupTimeout,
		StopTimeout:    stopTimeout,
	}
	var current *buildservice.Process
	var appDone <-chan error
	lastBinary := ""
	buildNumber := 0
	rebuild := func(reason string, changed []string) {
		buildNumber++
		started := time.Now()
		store.Update(buildservice.Snapshot{
			State:       buildservice.StateBuilding,
			Message:     reason,
			CommandPath: d.commandPath,
			WatchedRoot: d.root,
			BuildCount:  buildNumber,
		})
		assetMode := javaScriptAssetGenerationMode(d.root, changed)
		var assets generatedAssetResult
		var result buildservice.BuildResult
		var next *buildservice.Process
		var runOutput string
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
				startup := &startupOutput{}
				startApp := app
				startApp.Stdin = stdin
				startApp.Stdout = io.MultiWriter(startup.Stdout(), panelOutputWriter{store: store, stream: "stdout"})
				startApp.Stderr = io.MultiWriter(startup.Stderr(), panelOutputWriter{store: store, stream: "stderr"})
				var err error
				next, err = startApp.Start(ctx, result.Binary)
				if err != nil {
					runOutput = startup.String()
					return err
				}
				startup.Attach(stdout, stderr)
				return err
			})
		}))

		err := d.runProgress(tasks)
		output := combineOutput(assets.Output, result.Output)
		duration := time.Since(started)
		if err != nil {
			if assets.Err != nil {
				store.Update(buildservice.Snapshot{
					State:       buildservice.StateBuildFailed,
					Message:     fmt.Sprintf("JavaScript generation failed after %s.", assets.Duration.Round(time.Millisecond)),
					CommandPath: d.commandPath,
					WatchedRoot: d.root,
					BuildCount:  buildNumber,
					Duration:    assets.Duration.Round(time.Millisecond).String(),
					Output:      assets.Output,
				})
				return
			}
			if result.Err != nil {
				store.Update(buildservice.Snapshot{
					State:       buildservice.StateBuildFailed,
					Message:     fmt.Sprintf("Build failed after %s.", duration.Round(time.Millisecond)),
					CommandPath: d.commandPath,
					WatchedRoot: d.root,
					BuildCount:  buildNumber,
					Duration:    duration.Round(time.Millisecond).String(),
					Output:      output,
				})
				return
			}
			store.Update(buildservice.Snapshot{
				State:       buildservice.StateRunFailed,
				Message:     err.Error(),
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
				BuildCount:  buildNumber,
				Duration:    duration.Round(time.Millisecond).String(),
				Output:      runOutput,
			})
			if runOutput != "" {
				printOutput(d.stderr, runOutput)
			}
			return
		}

		old := current
		current = next
		appDone = next.Done()
		lastBinary = result.Binary
		server.SetTarget("http://" + next.Addr())
		store.Update(buildservice.Snapshot{
			State:            buildservice.StateRunning,
			Message:          "Application is running.",
			CommandPath:      d.commandPath,
			WatchedRoot:      d.root,
			BuildCount:       buildNumber,
			Duration:         duration.Round(time.Millisecond).String(),
			StartedAt:        time.Now(),
			AppAddr:          next.Addr(),
			ControlPlaneAddr: next.ControlPlaneAddr(),
			Changed:          changed,
		})
		if old != nil {
			old.Stop()
		}
		if buildNumber > 1 {
			server.BroadcastReload()
			store.AddEvent(buildservice.Event{Type: buildservice.EventReload, Message: "Browser reload broadcast.", Build: buildNumber})
		}
		fmt.Fprintf(d.stderr, "lazy: application running after %s\n", duration.Round(time.Millisecond))
	}

	rebuild("Starting initial build.", nil)

	for {
		select {
		case <-ctx.Done():
			server.ClearTarget()
			store.Update(buildservice.Snapshot{
				State:       buildservice.StateStopped,
				Message:     "lazy is shutting down.",
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
				BuildCount:  buildNumber,
			})
			if current != nil {
				stopProcessWithEscalation(current, d.forceKill)
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
			if onlyTailwindInputs(changed) && current != nil {
				result := d.buildTailwind(ctx)
				if result.Err != nil {
					output := result.Output
					if strings.TrimSpace(output) == "" {
						output = result.Err.Error() + "\n"
					}
					store.Update(buildservice.Snapshot{
						State:            buildservice.StateBuildFailed,
						Message:          fmt.Sprintf("Tailwind build failed after %s.", result.Duration.Round(time.Millisecond)),
						CommandPath:      d.commandPath,
						WatchedRoot:      d.root,
						BuildCount:       buildNumber,
						Duration:         result.Duration.Round(time.Millisecond).String(),
						Changed:          changed,
						Output:           output,
						AppAddr:          current.Addr(),
						ControlPlaneAddr: current.ControlPlaneAddr(),
					})
					continue
				}
				store.Update(buildservice.Snapshot{
					State:            buildservice.StateRunning,
					Message:          "Application is running.",
					CommandPath:      d.commandPath,
					WatchedRoot:      d.root,
					BuildCount:       buildNumber,
					Duration:         result.Duration.Round(time.Millisecond).String(),
					Changed:          changed,
					Output:           result.Output,
					AppAddr:          current.Addr(),
					ControlPlaneAddr: current.ControlPlaneAddr(),
				})
				server.BroadcastReload()
				store.AddEvent(buildservice.Event{Type: buildservice.EventReload, Message: "Tailwind rebuilt and browser reload broadcast.", Changed: changed, Build: buildNumber})
				fmt.Fprintf(d.stderr, "lazy: Tailwind rebuilt after %s\n", result.Duration.Round(time.Millisecond))
				continue
			}
			switch classifyDevelopmentChange(d.viewPath, d.publicPath, changed) {
			case devChangeReloadBrowser:
				if current != nil {
					store.Update(buildservice.Snapshot{
						State:            buildservice.StateRunning,
						Message:          "Application is running.",
						CommandPath:      d.commandPath,
						WatchedRoot:      d.root,
						BuildCount:       buildNumber,
						Changed:          changed,
						AppAddr:          current.Addr(),
						ControlPlaneAddr: current.ControlPlaneAddr(),
					})
					server.BroadcastReload()
					store.AddEvent(buildservice.Event{Type: buildservice.EventReload, Message: "Browser reload broadcast.", Changed: changed, Build: buildNumber})
					fmt.Fprintf(d.stderr, "lazy: browser reloaded after %d public file change(s)\n", len(changed))
					continue
				}
			case devChangeReloadViews:
				if current != nil {
					var result viewReloadResult
					err := d.runProgress(progress.Tasks{
						progress.Task("Reload views", func(_ io.Reader, _ io.Writer, stderr io.Writer) error {
							result = reloadViews(ctx, current.ControlPlaneAddr())
							if result.Err != nil && result.Output != "" {
								printOutput(stderr, result.Output)
							}
							return result.Err
						}),
					})
					if err != nil {
						output := result.Output
						if strings.TrimSpace(output) == "" {
							output = err.Error() + "\n"
						}
						store.Update(buildservice.Snapshot{
							State:            buildservice.StateReloadFailed,
							Message:          fmt.Sprintf("Reload views failed after %s.", result.Duration.Round(time.Millisecond)),
							CommandPath:      d.commandPath,
							WatchedRoot:      d.root,
							BuildCount:       buildNumber,
							Duration:         result.Duration.Round(time.Millisecond).String(),
							Changed:          changed,
							Output:           output,
							AppAddr:          current.Addr(),
							ControlPlaneAddr: current.ControlPlaneAddr(),
						})
						continue
					}
					store.Update(buildservice.Snapshot{
						State:            buildservice.StateRunning,
						Message:          "Application is running.",
						CommandPath:      d.commandPath,
						WatchedRoot:      d.root,
						BuildCount:       buildNumber,
						Duration:         result.Duration.Round(time.Millisecond).String(),
						Changed:          changed,
						AppAddr:          current.Addr(),
						ControlPlaneAddr: current.ControlPlaneAddr(),
					})
					server.BroadcastReload()
					store.AddEvent(buildservice.Event{Type: buildservice.EventReload, Message: "Views reloaded and browser reload broadcast.", Changed: changed, Build: buildNumber})
					fmt.Fprintf(d.stderr, "lazy: views reloaded after %s\n", result.Duration.Round(time.Millisecond))
					continue
				}
			}
			rebuild(fmt.Sprintf("Rebuilding after %d changed file(s).", len(changed)), changed)
		case request := <-actions:
			var err error
			switch request.Action {
			case buildservice.ActionRebuild:
				store.AddEvent(buildservice.Event{Type: buildservice.EventManual, Message: "Manual rebuild requested.", Build: buildNumber})
				rebuild("Manual rebuild requested.", nil)
			case buildservice.ActionRestart:
				store.AddEvent(buildservice.Event{Type: buildservice.EventManual, Message: "Manual restart requested.", Build: buildNumber})
				err = restartLatest(ctx, d, app, store, server, &current, &appDone, lastBinary, buildNumber)
			default:
				err = fmt.Errorf("unknown action %q", request.Action)
			}
			request.Reply <- err
		case err := <-appDone:
			appDone = nil
			current = nil
			server.ClearTarget()
			message := "Application exited."
			if err != nil {
				message = fmt.Sprintf("Application exited: %v", err)
			}
			store.Update(buildservice.Snapshot{
				State:       buildservice.StateRunFailed,
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
	return ctx.Err() != nil || err == nil || buildservice.Interrupted(err)
}

func serviceState(state lifecycleservice.State) buildservice.ServiceState {
	switch state {
	case lifecycleservice.StateReady:
		return buildservice.ServiceReady
	case lifecycleservice.StateStopped:
		return buildservice.ServiceStopped
	default:
		return buildservice.ServiceNotReady
	}
}

func waitForLifecycle(ctx context.Context, lifecycle *lifecycleservice.Manager) error {
	select {
	case err := <-lifecycle.Ready():
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func stopProcessWithEscalation(process *buildservice.Process, forceKill <-chan struct{}) {
	if process == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		process.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-forceKill:
		process.Kill()
		<-done
	}
}

func stopLifecycleWithEscalation(lifecycle *lifecycleservice.Manager, forceKill <-chan struct{}) {
	if lifecycle == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		lifecycle.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-forceKill:
		lifecycle.Kill()
		<-done
	}
}

func restartLatest(ctx context.Context, d *devRunner, app buildservice.Config, store *buildservice.Store, server *devserver.Server, current **buildservice.Process, appDone *<-chan error, binary string, buildNumber int) error {
	if binary == "" {
		return fmt.Errorf("no successful build is available to restart")
	}
	started := time.Now()
	store.Update(buildservice.Snapshot{
		State:       buildservice.StateStarting,
		Message:     "Restarting application.",
		CommandPath: d.commandPath,
		WatchedRoot: d.root,
		BuildCount:  buildNumber,
	})
	var next *buildservice.Process
	var runOutput string
	err := d.runProgress(progress.Tasks{
		progress.UITask("Restart application", func(ui *progress.UI) error {
			return ui.Takeover(func(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
				startup := &startupOutput{}
				startApp := app
				startApp.Stdin = stdin
				startApp.Stdout = io.MultiWriter(startup.Stdout(), panelOutputWriter{store: store, stream: "stdout"})
				startApp.Stderr = io.MultiWriter(startup.Stderr(), panelOutputWriter{store: store, stream: "stderr"})
				var err error
				next, err = startApp.Start(ctx, binary)
				if err != nil {
					runOutput = startup.String()
					return err
				}
				startup.Attach(stdout, stderr)
				return nil
			})
		}),
	})
	duration := time.Since(started)
	if err != nil {
		store.Update(buildservice.Snapshot{
			State:       buildservice.StateRunFailed,
			Message:     err.Error(),
			CommandPath: d.commandPath,
			WatchedRoot: d.root,
			BuildCount:  buildNumber,
			Duration:    duration.Round(time.Millisecond).String(),
			Output:      runOutput,
		})
		return err
	}
	old := *current
	*current = next
	*appDone = next.Done()
	server.SetTarget("http://" + next.Addr())
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		Message:          "Application restarted.",
		CommandPath:      d.commandPath,
		WatchedRoot:      d.root,
		BuildCount:       buildNumber,
		Duration:         duration.Round(time.Millisecond).String(),
		StartedAt:        time.Now(),
		AppAddr:          next.Addr(),
		ControlPlaneAddr: next.ControlPlaneAddr(),
	})
	if old != nil {
		old.Stop()
	}
	server.BroadcastReload()
	store.AddEvent(buildservice.Event{Type: buildservice.EventReload, Message: "Browser reload broadcast after restart.", Build: buildNumber})
	return nil
}

func (d *devRunner) runProgress(tasks progress.Tasks) error {
	return progress.Run(tasks, d.stdin, d.stdout, d.stderr)
}

func (d *devRunner) generateAssets(changed []string) generatedAssetResult {
	return d.generateAssetsForMode(javaScriptAssetGenerationMode(d.root, changed))
}

func reloadViews(ctx context.Context, addr string) viewReloadResult {
	started := time.Now()
	reloadCtx, cancel := context.WithTimeout(ctx, viewReloadTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(reloadCtx, http.MethodPost, "http://"+addr+lazyDevReloadViewsPath, nil)
	if err != nil {
		return viewReloadResult{Err: err, Duration: time.Since(started)}
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return viewReloadResult{Err: err, Duration: time.Since(started)}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return viewReloadResult{Err: err, Duration: time.Since(started)}
	}
	output := string(body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if strings.TrimSpace(output) == "" {
			output = response.Status + "\n"
		}
		return viewReloadResult{
			Output:   output,
			Err:      fmt.Errorf("reload views: %s", strings.TrimSpace(output)),
			Duration: time.Since(started),
		}
	}
	return viewReloadResult{Output: output, Duration: time.Since(started)}
}

func (d *devRunner) generateAssetsForMode(mode javaScriptAssetMode) generatedAssetResult {
	if mode == javaScriptAssetNone {
		return generatedAssetResult{}
	}

	switch mode {
	case javaScriptAssetFull:
		result := (jsservice.Service{Root: d.root}).Build(context.Background(), nil, nil)
		return generatedAssetResult{Output: result.Output, Err: result.Err, Duration: result.Duration}
	case javaScriptAssetBundle:
		result := (jsservice.Service{Root: d.root}).Bundle(context.Background())
		return generatedAssetResult{Output: result.Output, Err: result.Err, Duration: result.Duration}
	default:
		return generatedAssetResult{}
	}
}

func (d *devRunner) buildTailwind(ctx context.Context) generatedAssetResult {
	result := (tailwindservice.Service{Root: d.root}).Build(ctx, nil, nil)
	return generatedAssetResult{Output: result.Output, Err: result.Err, Duration: result.Duration}
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

func classifyDevelopmentChange(viewPath string, publicPath string, changed []string) devChangeAction {
	if len(changed) == 0 {
		return devChangeRebuild
	}
	hasView := false
	hasPublic := false
	for _, path := range changed {
		switch {
		case isActiveRootPath(path, viewPath, appcmd.DefaultViewPath):
			hasView = true
		case isActiveRootPath(path, publicPath, appcmd.DefaultPublicPath):
			hasPublic = true
		default:
			return devChangeRebuild
		}
	}
	if hasView {
		return devChangeReloadViews
	}
	if hasPublic {
		return devChangeReloadBrowser
	}
	return devChangeRebuild
}

func isActiveRootPath(path string, configuredRoot string, defaultRoot string) bool {
	root, ok := normalizedRelativeRoot(configuredRoot, defaultRoot)
	if !ok {
		return false
	}
	path = cleanWatchPath(path)
	return path == root || strings.HasPrefix(path, root+"/")
}

func normalizedRelativeRoot(configuredRoot string, defaultRoot string) (string, bool) {
	root := strings.TrimSpace(configuredRoot)
	if root == "" {
		root = defaultRoot
	}
	root = filepath.Clean(filepath.FromSlash(root))
	if filepath.IsAbs(root) || root == "." || strings.HasPrefix(root, ".."+string(filepath.Separator)) || root == ".." {
		return "", false
	}
	return filepath.ToSlash(root), true
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

func onlyTailwindInputs(changed []string) bool {
	if len(changed) == 0 {
		return false
	}
	for _, path := range changed {
		if !isTailwindInput(path) {
			return false
		}
	}
	return true
}

func isTailwindInput(path string) bool {
	path = cleanWatchPath(path)
	switch path {
	case "tailwind.config.js":
		return true
	}
	return strings.HasPrefix(path, "app/styles/") || strings.HasPrefix(path, "styles/")
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

func publicListenAddr(addr string, port int) string {
	normalizedAddr := normalizeListenAddr(addr)
	if port != 0 && (normalizedAddr == "" || normalizedAddr == defaultListenAddr) {
		return ":" + strconv.Itoa(port)
	}
	return normalizedAddr
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
