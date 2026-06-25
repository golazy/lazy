package jsservice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	jscommand "golazy.dev/lazy/commands/js"
)

type Result struct {
	Output   string
	Err      error
	Duration time.Duration
}

type Service struct {
	Root string
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
	code, err := (jscommand.Command{
		Dir:    s.Root,
		Stdout: stdout,
		Stderr: stderr,
	}).Execute()
	if err == nil && code != 0 {
		err = fmt.Errorf("lazy js failed with exit code %d", code)
	}
	if ctx.Err() != nil && err == nil {
		err = ctx.Err()
	}
	if output.Len() > 0 {
		return Result{Output: output.String(), Err: err, Duration: time.Since(started)}
	}
	return Result{Err: err, Duration: time.Since(started)}
}

func (s Service) Bundle(ctx context.Context) Result {
	started := time.Now()
	const output = "* Bundling JavaScript\n"
	manifest, err := jscommand.LoadManifest(s.Root)
	if err != nil {
		return Result{Output: output, Err: err, Duration: time.Since(started)}
	}
	packageDir := jscommand.PackageDir(s.Root, manifest)
	if _, err := jscommand.Bundle(manifest, s.Root, packageDir); err != nil {
		return Result{Output: output, Err: err, Duration: time.Since(started)}
	}
	if ctx.Err() != nil {
		return Result{Output: output, Err: ctx.Err(), Duration: time.Since(started)}
	}
	return Result{Output: output, Duration: time.Since(started)}
}
