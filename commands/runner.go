package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Options struct {
	Dir     string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Env     []string
	Capture bool
}

type Runner func(command string, args []string, options Options) error

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func Exec(command string, args []string, options Options) error {
	process := exec.Command(command, args...)
	process.Dir = options.Dir
	process.Stdin = options.Stdin
	if len(options.Env) != 0 {
		process.Env = append(os.Environ(), options.Env...)
	}

	var output bytes.Buffer
	if options.Capture {
		process.Stdout = &output
		process.Stderr = &output
	} else {
		process.Stdout = options.Stdout
		process.Stderr = options.Stderr
	}

	if err := process.Run(); err != nil {
		var processError *exec.ExitError
		if errors.As(err, &processError) && !options.Capture {
			return &ExitError{Code: processError.ExitCode(), Err: err}
		}
		if options.Capture && output.Len() > 0 {
			return fmt.Errorf("%w\n%s", err, strings.TrimSpace(output.String()))
		}
		return err
	}
	return nil
}
