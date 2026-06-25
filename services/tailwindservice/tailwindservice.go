package tailwindservice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	tailwindcommand "golazy.dev/lazy/commands/tailwind"
)

type Result struct {
	Output   string
	Err      error
	Duration time.Duration
}

type Service struct {
	Root   string
	Input  string
	Output string
}

func (s Service) Build(ctx context.Context, stdout io.Writer, stderr io.Writer) Result {
	started := time.Now()
	var output bytes.Buffer
	if stdout == nil {
		stdout = &output
	}
	if stderr == nil {
		stderr = &output
	}
	code, err := (tailwindcommand.Command{
		Dir:    s.Root,
		Input:  s.Input,
		Output: s.Output,
		Stdout: stdout,
		Stderr: stderr,
	}).Execute()
	if err == nil && code != 0 {
		err = fmt.Errorf("lazy tailwind failed with exit code %d", code)
	}
	if ctx.Err() != nil && err == nil {
		err = ctx.Err()
	}
	if output.Len() > 0 {
		return Result{Output: output.String(), Err: err, Duration: time.Since(started)}
	}
	return Result{Err: err, Duration: time.Since(started)}
}
