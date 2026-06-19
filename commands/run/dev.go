package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golazy/lazy/commands/run/devapp"
	"github.com/golazy/lazy/commands/run/reloadproxy"
	"github.com/golazy/lazy/commands/run/watcher"
)

const (
	defaultPollInterval   = 500 * time.Millisecond
	defaultDebounce       = 150 * time.Millisecond
	defaultStartupTimeout = 10 * time.Second
	stopTimeout           = 2 * time.Second
)

type devRunner struct {
	root        string
	commandPath string
	viewPath    string
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

	tmpDir, err := os.MkdirTemp("", "lazy-run-*")
	if err != nil {
		return 1, fmt.Errorf("create temporary build directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	proxy, err := reloadproxy.New(publicListenAddr())
	if err != nil {
		return 1, err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()
		_ = proxy.Shutdown(shutdownCtx)
	}()
	proxy.UpdateStatus(reloadproxy.Status{
		State:       reloadproxy.StateQueued,
		Message:     "Waiting for the first build.",
		CommandPath: d.commandPath,
		WatchedRoot: d.root,
	})
	if err := proxy.Start(); err != nil {
		return 1, err
	}

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
		proxy.UpdateStatus(reloadproxy.Status{
			State:       reloadproxy.StateBuilding,
			Message:     reason,
			CommandPath: d.commandPath,
			WatchedRoot: d.root,
			BuildCount:  buildNumber,
		})
		result := app.Build(ctx, tmpDir, buildNumber)
		if result.Output != "" {
			fmt.Fprint(d.stderr, result.Output)
			if !strings.HasSuffix(result.Output, "\n") {
				fmt.Fprintln(d.stderr)
			}
		}
		if result.Err != nil {
			proxy.UpdateStatus(reloadproxy.Status{
				State:       reloadproxy.StateBuildFailed,
				Message:     fmt.Sprintf("Build failed after %s.", result.Duration.Round(time.Millisecond)),
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
				BuildCount:  buildNumber,
				Duration:    result.Duration,
				Output:      result.Output,
			})
			fmt.Fprintf(d.stderr, "lazy: build failed after %s\n", result.Duration.Round(time.Millisecond))
			return
		}

		next, err := app.Start(ctx, result.Binary)
		if err != nil {
			proxy.UpdateStatus(reloadproxy.Status{
				State:       reloadproxy.StateRunFailed,
				Message:     err.Error(),
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
				BuildCount:  buildNumber,
				Duration:    result.Duration,
				Output:      result.Output,
			})
			fmt.Fprintf(d.stderr, "lazy: application failed to start: %v\n", err)
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
			Duration:    result.Duration,
			StartedAt:   time.Now(),
			Changed:     changed,
		})
		if old != nil {
			old.Stop()
		}
		if buildNumber > 1 {
			proxy.BroadcastReload()
		}
		fmt.Fprintf(d.stderr, "lazy: application running after %s\n", result.Duration.Round(time.Millisecond))
	}

	rebuild("Starting initial build.", nil)

	for {
		select {
		case <-ctx.Done():
			if current != nil {
				current.Stop()
			}
			proxy.ClearTarget()
			proxy.UpdateStatus(reloadproxy.Status{
				State:       reloadproxy.StateStopped,
				Message:     "lazy is shutting down.",
				CommandPath: d.commandPath,
				WatchedRoot: d.root,
				BuildCount:  buildNumber,
			})
			return 0, nil
		case changed, ok := <-changeCh:
			if !ok {
				changeCh = nil
				continue
			}
			changed = drainChanges(changeCh, changed)
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
		}
	}
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

func publicListenAddr() string {
	if addr := os.Getenv("ADDR"); addr != "" {
		return normalizeListenAddr(addr)
	}
	if port := os.Getenv("PORT"); port != "" {
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
