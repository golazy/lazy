package run

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/golazy/lazy/commands"
	"github.com/golazy/lazy/commands/appcmd"
)

type Command struct {
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Runner commands.Runner
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

	candidate, err := appcmd.Find(dir)
	if err != nil {
		return 1, err
	}

	runner := c.Runner
	if runner == nil {
		runner = commands.Exec
	}
	err = runner("go", []string{"run", "./" + filepath.ToSlash(candidate)}, commands.Options{
		Dir:    dir,
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
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
