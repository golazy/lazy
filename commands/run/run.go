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
)

type Command struct {
	Dir      string
	CmdPath  string
	ViewPath string
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	Runner   commands.Runner
	Context  context.Context
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
		stdin:       c.Stdin,
		stdout:      c.Stdout,
		stderr:      c.Stderr,
	}).run(ctx)
}

func (c Command) executeDirect(dir string, candidate string, runner commands.Runner) (int, error) {
	env, err := appcmd.ViewPathEnv(dir, c.ViewPath)
	if err != nil {
		return 1, err
	}
	err = runner("go", appcmd.GoRunArgs("lazydev", filepath.ToSlash(candidate)), commands.Options{
		Dir:    dir,
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
		Env:    env,
	})
	if err == nil {
		return 0, nil
	}

	var processExit *commands.ExitError
	if errors.As(err, &processExit) {
		return processExit.Code, nil
	}
	return 1, fmt.Errorf("run application: %w", err)
}
