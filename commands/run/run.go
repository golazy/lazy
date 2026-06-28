package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"golazy.dev/lazy/services/appservice"
	"golazy.dev/lazy/services/configservice"
	"golazy.dev/lazy/services/devloopservice"
	"golazy.dev/lazy/services/execservice"
	"golazy.dev/lazy/services/workspaceservice"
	"golazy.dev/lazytui/progress"
)

type Command struct {
	Dir        string
	CmdPath    string
	ViewPath   string
	PublicPath string
	Addr       string
	Port       int
	GoWork     string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Runner     execservice.Runner
	Context    context.Context
}

func (c Command) Execute() (int, error) {
	dir := c.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return 1, fmt.Errorf("get working directory: %w", err)
		}
	}

	candidate, err := appservice.Find(dir, c.CmdPath)
	if err != nil {
		return 1, err
	}

	runner := c.Runner
	if runner != nil {
		return c.executeDirect(dir, candidate, runner)
	}

	serviceConfig, _, err := configservice.LoadIfExists(dir)
	if err != nil {
		return 1, err
	}

	ctx := c.Context
	var stop func()
	var forceKill <-chan struct{}
	if ctx == nil {
		ctx, stop, forceKill = interruptContext()
		defer stop()
	}
	return (devloopservice.Config{
		Root:          dir,
		CommandPath:   candidate,
		ViewPath:      c.ViewPath,
		PublicPath:    c.PublicPath,
		ListenAddr:    devloopservice.PublicListenAddr(c.Addr, c.Port),
		GoWork:        c.GoWork,
		ServiceConfig: serviceConfig,
		Stdin:         c.Stdin,
		Stdout:        c.Stdout,
		Stderr:        c.Stderr,
		ForceKill:     forceKill,
	}).Run(ctx)
}

func interruptContext() (context.Context, func(), <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 2)
	done := make(chan struct{})
	forceKill := make(chan struct{})
	var doneOnce sync.Once
	var killOnce sync.Once
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		interrupted := false
		for {
			select {
			case <-done:
				return
			case <-signals:
				if !interrupted {
					interrupted = true
					cancel()
					continue
				}
				killOnce.Do(func() {
					close(forceKill)
				})
				signal.Stop(signals)
				return
			}
		}
	}()

	stop := func() {
		doneOnce.Do(func() {
			close(done)
			signal.Stop(signals)
			cancel()
		})
	}
	return ctx, stop, forceKill
}

func (c Command) executeDirect(dir string, candidate string, runner execservice.Runner) (int, error) {
	buildFlags, err := appservice.LazyDevBuildFlags(dir, c.ViewPath, c.PublicPath)
	if err != nil {
		return 1, err
	}
	workspaceActive, err := workspaceservice.Active(dir, c.GoWork)
	if err != nil {
		return 1, fmt.Errorf("inspect Go workspace: %w", err)
	}
	tasks := progress.Tasks{}
	if !workspaceActive {
		tasks = append(tasks, progress.Task("Update Go modules", func(_ io.Reader, _ io.Writer, _ io.Writer) error {
			if err := runner("go", []string{"mod", "tidy"}, execservice.Options{
				Dir:     dir,
				Capture: true,
			}); err != nil {
				return fmt.Errorf("go mod tidy: %w", err)
			}
			return nil
		}))
	}
	tasks = append(tasks, progress.UITask("Run application", func(ui *progress.UI) error {
		return ui.Takeover(func(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
			if err := runner("go", appservice.GoRunArgs("lazydev", filepath.ToSlash(candidate), buildFlags...), execservice.Options{
				Dir:    dir,
				Stdin:  stdin,
				Stdout: stdout,
				Stderr: stderr,
			}); err != nil {
				return fmt.Errorf("run application: %w", err)
			}
			return nil
		})
	}))

	if err := c.runProgress(tasks); err != nil {
		var processExit *execservice.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, err
	}
	return 0, nil
}

func (c Command) stdout() io.Writer {
	if c.Stdout == nil {
		return io.Discard
	}
	return c.Stdout
}

func (c Command) stderr() io.Writer {
	if c.Stderr == nil {
		return io.Discard
	}
	return c.Stderr
}

func (c Command) runProgress(tasks progress.Tasks) error {
	return progress.Run(tasks, c.Stdin, c.stdout(), c.stderr())
}
