package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/appcmd"
	"golazy.dev/lazy/commands/gowork"
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
	Runner     commands.Runner
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

	candidate, err := appcmd.Find(dir, c.CmdPath)
	if err != nil {
		return 1, err
	}

	runner := c.Runner
	if runner != nil {
		return c.executeDirect(dir, candidate, runner)
	}

	ctx := c.Context
	var stop context.CancelFunc
	if ctx == nil {
		ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
	}
	return (&devRunner{
		root:        dir,
		commandPath: candidate,
		viewPath:    c.ViewPath,
		publicPath:  c.PublicPath,
		listenAddr:  publicListenAddr(c.Addr, c.Port),
		goWork:      c.GoWork,
		stdin:       c.Stdin,
		stdout:      c.Stdout,
		stderr:      c.Stderr,
	}).run(ctx)
}

func (c Command) executeDirect(dir string, candidate string, runner commands.Runner) (int, error) {
	buildFlags, err := appcmd.LazyDevBuildFlags(dir, c.ViewPath, c.PublicPath)
	if err != nil {
		return 1, err
	}
	workspaceActive, err := gowork.Active(dir, c.GoWork)
	if err != nil {
		return 1, fmt.Errorf("inspect Go workspace: %w", err)
	}
	tasks := progress.Tasks{}
	if !workspaceActive {
		tasks = append(tasks, progress.Task("Update Go modules", func(_ io.Reader, _ io.Writer, _ io.Writer) error {
			if err := runner("go", []string{"mod", "tidy"}, commands.Options{
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
			if err := runner("go", appcmd.GoRunArgs("lazydev", filepath.ToSlash(candidate), buildFlags...), commands.Options{
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
		var processExit *commands.ExitError
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
