package datasetservice

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"golazy.dev/lazy/services/configservice"
	"golazy.dev/lazy/services/execservice"
	"golazy.dev/lazy/services/taskservice"
)

var datasetNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Command struct {
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Runner execservice.Runner
}

func (c Command) Dump(name string) error {
	dir := c.dir()
	datasetDir, err := datasetPath(dir, name)
	if err != nil {
		return err
	}
	config, _, err := configservice.LoadIfExists(dir)
	if err != nil {
		return err
	}
	inventory, err := taskservice.Inspect(dir, config)
	if err != nil {
		return err
	}
	if len(inventory.Services) == 0 {
		return fmt.Errorf("no services found; declare services in lazy.toml or add .mise/tasks/<service>/start")
	}

	var dumpable []string
	for _, service := range inventory.Services {
		if taskservice.HasTask(inventory.Tasks, service, "dump") {
			dumpable = append(dumpable, service)
		}
	}
	if len(dumpable) == 0 {
		return fmt.Errorf("no discovered services define a dump task")
	}
	if err := os.MkdirAll(datasetDir, 0o755); err != nil {
		return fmt.Errorf("create dataset directory: %w", err)
	}
	for _, service := range dumpable {
		path, err := serviceDumpPath(datasetDir, service)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.stderr(), "lazy: dumping %s to %s\n", service, filepath.ToSlash(path))
		if err := taskservice.RunTask(c.Runner, dir, taskservice.TaskName(service, "dump"), []string{path}, c.Stdin, c.Stdout, c.Stderr, false); err != nil {
			return fmt.Errorf("%s:dump: %w", service, err)
		}
	}
	return nil
}

func (c Command) Load(name string) error {
	dir := c.dir()
	datasetDir, err := datasetPath(dir, name)
	if err != nil {
		return err
	}
	if info, err := os.Stat(datasetDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("dataset %q does not exist", name)
		}
		return fmt.Errorf("inspect dataset: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("dataset %q is not a directory", name)
	}

	config, _, err := configservice.LoadIfExists(dir)
	if err != nil {
		return err
	}
	inventory, err := taskservice.Inspect(dir, config)
	if err != nil {
		return err
	}
	if len(inventory.Services) == 0 {
		return fmt.Errorf("no services found; declare services in lazy.toml or add .mise/tasks/<service>/start")
	}

	loaded := 0
	for _, service := range inventory.Services {
		path, err := serviceDumpPath(datasetDir, service)
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect %s: %w", filepath.ToSlash(path), err)
		}
		if !taskservice.HasTask(inventory.Tasks, service, "load") {
			return fmt.Errorf("dataset includes %s but %s has no load task", filepath.ToSlash(path), service)
		}
		fmt.Fprintf(c.stderr(), "lazy: loading %s from %s\n", service, filepath.ToSlash(path))
		if err := taskservice.RunTask(c.Runner, dir, taskservice.TaskName(service, "load"), []string{path}, c.Stdin, c.Stdout, c.Stderr, false); err != nil {
			return fmt.Errorf("%s:load: %w", service, err)
		}
		loaded++
	}
	if loaded == 0 {
		return fmt.Errorf("dataset %q has no dumps for discovered services", name)
	}
	return nil
}

func (c Command) dir() string {
	if c.Dir != "" {
		return c.Dir
	}
	return "."
}

func (c Command) stderr() io.Writer {
	if c.Stderr == nil {
		return io.Discard
	}
	return c.Stderr
}

func datasetPath(dir string, name string) (string, error) {
	if !datasetNamePattern.MatchString(name) || name == "." || name == ".." {
		return "", fmt.Errorf("dataset name must be a single path-safe name")
	}
	return filepath.Join(dir, "datasets", name), nil
}

func serviceDumpPath(datasetDir string, service string) (string, error) {
	if !datasetNamePattern.MatchString(service) || service == "." || service == ".." {
		return "", fmt.Errorf("service name %q must be a single path-safe name for datasets", service)
	}
	return filepath.Join(datasetDir, service+".dump"), nil
}
