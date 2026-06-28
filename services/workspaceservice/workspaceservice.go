package workspaceservice

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Active reports whether Go workspace mode points at a go.work file.
func Active(dir string, goWork string) (bool, error) {
	if strings.TrimSpace(goWork) != "" {
		return valueActive(goWork), nil
	}

	value, err := GoEnv(dir)
	if err != nil {
		return false, err
	}
	return valueActive(value), nil
}

func GoEnv(dir string) (string, error) {
	command := exec.Command("go", "env", "GOWORK")
	command.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return "", fmt.Errorf("go env GOWORK: %w\n%s", err, message)
		}
		return "", fmt.Errorf("go env GOWORK: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func valueActive(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.EqualFold(value, "off")
}
