package execservice

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

type OutputRunner func(command string, args []string, options Options) ([]byte, error)

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

func ExecOutput(command string, args []string, options Options) ([]byte, error) {
	process := exec.Command(command, args...)
	process.Dir = options.Dir
	process.Stdin = options.Stdin
	if len(options.Env) != 0 {
		process.Env = append(os.Environ(), options.Env...)
	}

	var output bytes.Buffer
	process.Stdout = &output
	process.Stderr = options.Stderr

	if err := process.Run(); err != nil {
		var processError *exec.ExitError
		if errors.As(err, &processError) {
			return output.Bytes(), &ExitError{Code: processError.ExitCode(), Err: err}
		}
		return output.Bytes(), err
	}
	return output.Bytes(), nil
}

func ResolveMiseCommand() (string, []string) {
	if _, err := exec.LookPath("mise"); err == nil {
		return "mise", nil
	}

	for _, candidate := range miseCommandCandidates() {
		if !isExecutableFile(candidate) {
			continue
		}
		dir := filepath.Dir(candidate)
		return candidate, []string{"PATH=" + prependPathDir(os.Getenv("PATH"), dir)}
	}

	return "mise", nil
}

func MiseExecCommand(command string, args []string) (string, []string, []string) {
	miseCommand, miseEnv := ResolveMiseCommand()
	return miseCommand, append([]string{"exec", "--", command}, args...), miseEnv
}

func MiseExecRunnerCommand(runner Runner, command string, args []string) (string, []string, []string) {
	if runner == nil {
		return MiseExecCommand(command, args)
	}
	return "mise", append([]string{"exec", "--", command}, args...), nil
}

func MiseExecOutputRunnerCommand(runner OutputRunner, command string, args []string) (string, []string, []string) {
	if runner == nil {
		return MiseExecCommand(command, args)
	}
	return "mise", append([]string{"exec", "--", command}, args...), nil
}

func miseCommandCandidates() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "bin", executableName("mise")),
	}
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

func prependPathDir(pathValue string, dir string) string {
	if dir == "" {
		return pathValue
	}
	for _, entry := range filepath.SplitList(pathValue) {
		if entry == dir {
			return pathValue
		}
	}
	if pathValue == "" {
		return dir
	}
	return dir + string(os.PathListSeparator) + pathValue
}
