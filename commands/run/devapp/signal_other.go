//go:build !unix

package devapp

import (
	"errors"
	"os/exec"
)

func Interrupted(err error) bool {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	return exit.ExitCode() == 130 || exit.ExitCode() == 143
}
