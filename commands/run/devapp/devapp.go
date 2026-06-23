package devapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"golazy.dev/lazy/commands/appcmd"
)

const defaultStopTimeout = 2 * time.Second

type Config struct {
	Root           string
	CommandPath    string
	ViewPath       string
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	StartupTimeout time.Duration
	StopTimeout    time.Duration
}

type BuildResult struct {
	Binary   string
	Output   string
	Err      error
	Duration time.Duration
}

type Process struct {
	command     *exec.Cmd
	addr        string
	done        chan error
	stopTimeout time.Duration
}

func (c Config) Build(ctx context.Context, tmpDir string, buildNumber int) BuildResult {
	started := time.Now()
	binary := filepath.Join(tmpDir, "app-"+strconv.Itoa(buildNumber)+exeSuffix())

	var output bytes.Buffer
	tidy := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidy.Dir = c.Root
	tidy.Stdout = &output
	tidy.Stderr = &output

	err := tidy.Run()
	if err == nil {
		args := appcmd.GoBuildArgs("lazydev", filepath.ToSlash(c.CommandPath), binary)
		build := exec.CommandContext(ctx, "go", args...)
		build.Dir = c.Root
		build.Stdout = &output
		build.Stderr = &output
		err = build.Run()
	}
	return BuildResult{
		Binary:   binary,
		Output:   output.String(),
		Err:      err,
		Duration: time.Since(started),
	}
}

func (c Config) Start(ctx context.Context, binary string) (*Process, error) {
	addr, err := freeLoopbackAddr()
	if err != nil {
		return nil, err
	}
	viewPathEnv, err := appcmd.ViewPathEnv(c.Root, c.ViewPath)
	if err != nil {
		return nil, err
	}

	stopTimeout := stopTimeoutOrDefault(c.StopTimeout)
	cmd := exec.CommandContext(ctx, binary)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := cmd.Process.Signal(os.Interrupt)
		if errors.Is(err, os.ErrProcessDone) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = stopTimeout
	cmd.Dir = c.Root
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Env = append(os.Environ(), append(viewPathEnv, "ADDR="+addr)...)

	done := make(chan error, 1)
	process := &Process{
		command:     cmd,
		addr:        addr,
		done:        done,
		stopTimeout: stopTimeout,
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		done <- cmd.Wait()
		close(done)
	}()

	if err := waitForTCP(ctx, addr, done, c.StartupTimeout); err != nil {
		process.Stop()
		return nil, err
	}
	return process, nil
}

func (p *Process) Addr() string {
	return p.addr
}

func (p *Process) Done() <-chan error {
	return p.done
}

func (p *Process) Stop() {
	if p == nil || p.command == nil || p.command.Process == nil {
		return
	}
	stopTimeout := stopTimeoutOrDefault(p.stopTimeout)
	_ = p.command.Process.Signal(os.Interrupt)
	select {
	case <-p.done:
		return
	case <-time.After(stopTimeout):
		_ = p.command.Process.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(stopTimeout):
	}
}

func waitForTCP(ctx context.Context, addr string, done <-chan error, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			if err == nil {
				return errors.New("application exited before accepting connections")
			}
			return fmt.Errorf("application exited before accepting connections: %w", err)
		case <-deadline.C:
			return fmt.Errorf("application did not listen on %s within %s", addr, timeout)
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

func freeLoopbackAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("find free loopback port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().String(), nil
}

func stopTimeoutOrDefault(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	return defaultStopTimeout
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
