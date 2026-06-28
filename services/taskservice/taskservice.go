package taskservice

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golazy.dev/lazy/services/configservice"
	"golazy.dev/lazy/services/execservice"
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
	Config        configservice.Config
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	Runner        execservice.Runner
	CheckTimeout  time.Duration
	CheckInterval time.Duration
}

func Inspect(dir string, config configservice.Config) (Inventory, error) {
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
			discoverListedTasks(dir, tasks)
			return tasks, nil
		}
		return nil, fmt.Errorf("inspect mise tasks: %w", err)
	}
	discoverListedTasks(dir, tasks)
	return tasks, nil
}

func discoverListedTasks(dir string, tasks TaskSet) {
	command, env := execservice.ResolveMiseCommand()
	cmd := exec.Command(command, "tasks", "ls", "--all")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, "MISE_NO_ENV=1")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if !visibleTaskName(name) {
			continue
		}
		tasks[name] = struct{}{}
	}
}

func visibleTaskName(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return false
	}
	for _, part := range strings.Split(name, ":") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") {
			return false
		}
	}
	return true
}

func HasTask(tasks TaskSet, service string, action string) bool {
	_, ok := tasks[TaskName(service, action)]
	return ok
}

func TaskName(service string, action string) string {
	return service + ":" + action
}

func RunTask(runner execservice.Runner, dir string, task string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, capture bool) error {
	injected := runner != nil
	if runner == nil {
		runner = execservice.Exec
	}
	command, runArgs, env := miseRunCommand(injected, task, args)
	return runner(command, runArgs, execservice.Options{
		Dir:     dir,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Env:     env,
		Capture: capture,
	})
}

func TaskCommand(task string, args []string) (string, []string, []string) {
	return miseRunCommand(false, task, args)
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

func configServiceNames(config configservice.Config) []string {
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
	command, env := execservice.ResolveMiseCommand()
	return command, runArgs, env
}
