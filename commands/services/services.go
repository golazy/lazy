package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/lazyconfig"
)

const (
	defaultCheckTimeout  = 30 * time.Second
	defaultCheckInterval = 500 * time.Millisecond
)

type TaskSet map[string]struct{}

type Inventory struct {
	Services []string
	Tasks    TaskSet
}

type Preparer struct {
	Dir           string
	Config        lazyconfig.Config
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	Runner        commands.Runner
	CheckTimeout  time.Duration
	CheckInterval time.Duration
}

func Inspect(dir string, config lazyconfig.Config) (Inventory, error) {
	if dir == "" {
		dir = "."
	}
	tasks, err := DiscoverTasks(dir)
	if err != nil {
		return Inventory{}, err
	}

	names := configServiceNames(config)
	if len(names) == 0 {
		names = servicesWithStartTasks(tasks)
	}

	return Inventory{
		Services: names,
		Tasks:    tasks,
	}, nil
}

func DiscoverTasks(dir string) (TaskSet, error) {
	root := filepath.Join(dir, ".mise", "tasks")
	tasks := TaskSet{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		task := taskName(rel)
		if task != "" {
			tasks[task] = struct{}{}
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return tasks, nil
		}
		return nil, fmt.Errorf("inspect mise tasks: %w", err)
	}
	return tasks, nil
}

func HasTask(tasks TaskSet, service string, action string) bool {
	_, ok := tasks[TaskName(service, action)]
	return ok
}

func TaskName(service string, action string) string {
	return service + ":" + action
}

func RunTask(runner commands.Runner, dir string, task string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, capture bool) error {
	injected := runner != nil
	if runner == nil {
		runner = commands.Exec
	}
	command, runArgs, env := miseRunCommand(injected, task, args)
	return runner(command, runArgs, commands.Options{
		Dir:     dir,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Env:     env,
		Capture: capture,
	})
}

func (p Preparer) Execute() error {
	dir := p.Dir
	if dir == "" {
		dir = "."
	}
	inventory, err := Inspect(dir, p.Config)
	if err != nil {
		return err
	}
	for _, service := range inventory.Services {
		if HasTask(inventory.Tasks, service, "check") {
			if err := p.waitForCheck(service); err != nil {
				return err
			}
		}
		if HasTask(inventory.Tasks, service, "create") {
			if err := RunTask(p.Runner, dir, TaskName(service, "create"), nil, p.Stdin, p.Stdout, p.Stderr, false); err != nil {
				return fmt.Errorf("%s:create: %w", service, err)
			}
		}
		if HasTask(inventory.Tasks, service, "migrate") {
			if err := RunTask(p.Runner, dir, TaskName(service, "migrate"), nil, p.Stdin, p.Stdout, p.Stderr, false); err != nil {
				return fmt.Errorf("%s:migrate: %w", service, err)
			}
		}
	}
	return nil
}

func (p Preparer) waitForCheck(service string) error {
	dir := p.Dir
	if dir == "" {
		dir = "."
	}
	timeout := p.CheckTimeout
	if timeout <= 0 {
		timeout = defaultCheckTimeout
	}
	interval := p.CheckInterval
	if interval <= 0 {
		interval = defaultCheckInterval
	}

	started := time.Now()
	var lastErr error
	for {
		err := RunTask(p.Runner, dir, TaskName(service, "check"), nil, p.Stdin, io.Discard, p.Stderr, true)
		if err == nil {
			return nil
		}
		lastErr = err
		if time.Since(started) >= timeout {
			return fmt.Errorf("%s:check did not succeed within %s: %w", service, timeout.Round(time.Millisecond), lastErr)
		}
		time.Sleep(interval)
	}
}

func configServiceNames(config lazyconfig.Config) []string {
	if len(config.Services) == 0 {
		return nil
	}
	names := make([]string, 0, len(config.Services))
	for _, service := range config.Services {
		if service.Name != "" {
			names = append(names, service.Name)
		}
	}
	return names
}

func servicesWithStartTasks(tasks TaskSet) []string {
	seen := map[string]struct{}{}
	for task := range tasks {
		service, action, ok := strings.Cut(task, ":")
		if !ok || service == "" || action != "start" {
			continue
		}
		seen[service] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func taskName(rel string) string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	ext := filepath.Ext(last)
	if ext != "" {
		last = strings.TrimSuffix(last, ext)
	}
	if last == "" || strings.HasPrefix(last, "_") {
		return ""
	}
	parts[len(parts)-1] = last
	return strings.Join(parts, ":")
}

func miseRunCommand(injected bool, task string, args []string) (string, []string, []string) {
	runArgs := []string{"run", task}
	if len(args) > 0 {
		runArgs = append(runArgs, "--")
		runArgs = append(runArgs, args...)
	}
	if injected {
		return "mise", runArgs, nil
	}
	command, env := commands.ResolveMiseCommand()
	return command, runArgs, env
}
