package appcmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

func Find(dir string) (string, error) {
	moduleName, err := ModuleName(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}

	appName := filepath.Base(moduleName)
	candidates := []string{
		filepath.Join("cmd", appName),
		filepath.Join("cmd", "app"),
	}

	for _, candidate := range candidates {
		if isDirectory(filepath.Join(dir, candidate)) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(
		"application command not found; tried ./cmd/%s and ./cmd/app",
		appName,
	)
}

func ModuleName(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("go.mod not found")
		}
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	module := modfile.ModulePath(data)
	if module == "" {
		return "", fmt.Errorf("go.mod does not declare a module")
	}
	return module, nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
