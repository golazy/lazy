//go:build unix

package buildservice

import (
	"errors"
	"os/exec"
	"syscall"
)

func Interrupted(err error) bool {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	if exit.ExitCode() == 130 || exit.ExitCode() == 143 {
		return true
	}
	if exit.ProcessState == nil {
		return false
	}
	status, ok := exit.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return false
	}
	switch status.Signal() {
	case syscall.SIGINT, syscall.SIGTERM:
		return true
	default:
		return false
	}
}
