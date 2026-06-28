package lifecycleservice

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golazy.dev/lazy/commands/lazyconfig"
	commandservices "golazy.dev/lazy/commands/services"
)

const (
	defaultCheckInterval     = 500 * time.Millisecond
	defaultCheckWarningDelay = 5 * time.Second
	defaultStopTimeout       = 2 * time.Second
)

type TaskRunner func(ctx context.Context, dir string, task string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, capture bool) error

type Starter func(dir string, task string, stdout io.Writer, stderr io.Writer, stopTimeout time.Duration) (Process, error)

type Process interface {
	Done() <-chan error
	Stop()
	Kill()
}

type Service struct {
	Dir               string
	Config            lazyconfig.Config
	Stdin             io.Reader
	Stdout            io.Writer
	Stderr            io.Writer
	Runner            TaskRunner
	Starter           Starter
	CheckInterval     time.Duration
	CheckWarningDelay time.Duration
	StopTimeout       time.Duration
}

type Manager struct {
	services    []string
	ready       chan error
	processesMu sync.Mutex
	processes   map[string]Process
	stopping    bool
}

func (s Service) Start(ctx context.Context) (*Manager, error) {
	dir := s.Dir
	if dir == "" {
		dir = "."
	}
	inventory, err := commandservices.Inspect(dir, s.Config)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		services:  append([]string(nil), inventory.Services...),
		ready:     make(chan error, 1),
		processes: map[string]Process{},
	}
	if len(inventory.Services) == 0 {
		manager.ready <- nil
		return manager, nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(inventory.Services))
	for _, service := range inventory.Services {
		service := service
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.startService(ctx, manager, inventory, service); err != nil {
				errs <- err
			}
		}()
	}
	go func() {
		wg.Wait()
		close(errs)
		var joined []error
		for err := range errs {
			if err != nil {
				joined = append(joined, err)
			}
		}
		manager.ready <- errors.Join(joined...)
	}()

	return manager, nil
}

func (m *Manager) Ready() <-chan error {
	if m == nil {
		ready := make(chan error, 1)
		ready <- nil
		return ready
	}
	return m.ready
}

func (m *Manager) Len() int {
	if m == nil {
		return 0
	}
	return len(m.services)
}

func (m *Manager) Stop() {
	if m == nil {
		return
	}
	processes := m.snapshotProcesses(true)
	var wg sync.WaitGroup
	for _, process := range processes {
		process := process
		wg.Add(1)
		go func() {
			defer wg.Done()
			process.Stop()
		}()
	}
	wg.Wait()
}

func (m *Manager) Kill() {
	if m == nil {
		return
	}
	processes := m.snapshotProcesses(true)
	for _, process := range processes {
		process.Kill()
	}
}

func (m *Manager) addProcess(name string, process Process) bool {
	m.processesMu.Lock()
	defer m.processesMu.Unlock()
	if m.stopping {
		return false
	}
	m.processes[name] = process
	return true
}

func (m *Manager) removeProcess(name string, process Process) {
	m.processesMu.Lock()
	defer m.processesMu.Unlock()
	if m.processes[name] == process {
		delete(m.processes, name)
	}
}

func (m *Manager) isStopping() bool {
	m.processesMu.Lock()
	defer m.processesMu.Unlock()
	return m.stopping
}

func (m *Manager) snapshotProcesses(stopping bool) []Process {
	m.processesMu.Lock()
	defer m.processesMu.Unlock()
	if stopping {
		m.stopping = true
	}
	processes := make([]Process, 0, len(m.processes))
	for _, process := range m.processes {
		processes = append(processes, process)
	}
	return processes
}

func (s Service) startService(ctx context.Context, manager *Manager, inventory commandservices.Inventory, service string) error {
	startTask := commandservices.TaskName(service, "start")
	s.logf("lazy: starting %s service\n", service)
	process, err := s.starter()(s.dir(), startTask, s.stdout(), s.stderr(), s.stopTimeout())
	if err != nil {
		return fmt.Errorf("%s:start: %w", service, err)
	}
	if !manager.addProcess(service, process) {
		process.Stop()
		return context.Canceled
	}

	if commandservices.HasTask(inventory.Tasks, service, "check") {
		if err := s.waitForCheck(ctx, manager, service, process); err != nil {
			return err
		}
	}

	for _, action := range []string{"create", "migrate"} {
		if !commandservices.HasTask(inventory.Tasks, service, action) {
			continue
		}
		task := commandservices.TaskName(service, action)
		s.logf("lazy: running %s\n", task)
		if err := s.runner()(ctx, s.dir(), task, nil, s.Stdin, s.stdout(), s.stderr(), false); err != nil {
			s.logf("lazy: %s failed: %v\n", task, err)
			continue
		}
		s.logf("lazy: %s finished\n", task)
	}

	go s.watchProcess(manager, service, process)
	return nil
}

func (s Service) waitForCheck(ctx context.Context, manager *Manager, service string, process Process) error {
	task := commandservices.TaskName(service, "check")
	interval := s.CheckInterval
	if interval <= 0 {
		interval = defaultCheckInterval
	}
	warningDelay := s.CheckWarningDelay
	if warningDelay <= 0 {
		warningDelay = defaultCheckWarningDelay
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	warning := time.NewTimer(warningDelay)
	defer warning.Stop()

	warned := false
	for {
		err := s.runner()(ctx, s.dir(), task, nil, s.Stdin, io.Discard, s.stderr(), true)
		if err == nil {
			s.logf("lazy: %s is ready\n", service)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case processErr := <-process.Done():
			manager.removeProcess(service, process)
			if processErr == nil {
				return fmt.Errorf("%s:start exited before %s succeeded", service, task)
			}
			return fmt.Errorf("%s:start exited before %s succeeded: %w", service, task, processErr)
		case <-warning.C:
			if !warned {
				s.logf("lazy: %s is still failing after %s; check the service output. lazy will keep checking.\n", task, warningDelay.Round(time.Millisecond))
				if message := strings.TrimSpace(err.Error()); message != "" {
					s.logf("lazy: latest %s error: %s\n", task, message)
				}
				warned = true
			}
		case <-ticker.C:
		}
	}
}

func (s Service) watchProcess(manager *Manager, service string, process Process) {
	err := <-process.Done()
	manager.removeProcess(service, process)
	if manager.isStopping() {
		return
	}
	if err != nil {
		s.logf("lazy: %s service exited: %v\n", service, err)
		return
	}
	s.logf("lazy: %s service exited\n", service)
}

func (s Service) dir() string {
	if s.Dir == "" {
		return "."
	}
	return s.Dir
}

func (s Service) stdout() io.Writer {
	if s.Stdout == nil {
		return io.Discard
	}
	return s.Stdout
}

func (s Service) stderr() io.Writer {
	if s.Stderr == nil {
		return io.Discard
	}
	return s.Stderr
}

func (s Service) stopTimeout() time.Duration {
	if s.StopTimeout > 0 {
		return s.StopTimeout
	}
	return defaultStopTimeout
}

func (s Service) runner() TaskRunner {
	if s.Runner != nil {
		return s.Runner
	}
	return runTask
}

func (s Service) starter() Starter {
	if s.Starter != nil {
		return s.Starter
	}
	return startTask
}

func (s Service) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(s.stderr(), format, args...)
}

func runTask(ctx context.Context, dir string, task string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, capture bool) error {
	command, runArgs, env := commandservices.TaskCommand(task, args)
	cmd := exec.CommandContext(ctx, command, runArgs...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	if len(env) != 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	var output bytes.Buffer
	if capture {
		cmd.Stdout = &output
		cmd.Stderr = &output
	} else {
		cmd.Stdout = stdout
		cmd.Stderr = stderr
	}

	if err := cmd.Run(); err != nil {
		if capture && output.Len() > 0 {
			return fmt.Errorf("%w\n%s", err, strings.TrimSpace(output.String()))
		}
		return err
	}
	return nil
}

type taskProcess struct {
	command     *exec.Cmd
	done        chan error
	stopTimeout time.Duration
}

func startTask(dir string, task string, stdout io.Writer, stderr io.Writer, stopTimeout time.Duration) (Process, error) {
	command, args, env := commandservices.TaskCommand(task, nil)
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if len(env) != 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	done := make(chan error, 1)
	process := &taskProcess{
		command:     cmd,
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
	return process, nil
}

func (p *taskProcess) Done() <-chan error {
	if p == nil {
		return nil
	}
	return p.done
}

func (p *taskProcess) Stop() {
	if p == nil || p.command == nil || p.command.Process == nil {
		return
	}
	_ = p.command.Process.Signal(os.Interrupt)
	select {
	case <-p.done:
		return
	case <-time.After(p.stopTimeoutOrDefault()):
		p.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(p.stopTimeoutOrDefault()):
	}
}

func (p *taskProcess) Kill() {
	if p == nil || p.command == nil || p.command.Process == nil {
		return
	}
	_ = p.command.Process.Kill()
}

func (p *taskProcess) stopTimeoutOrDefault() time.Duration {
	if p.stopTimeout > 0 {
		return p.stopTimeout
	}
	return defaultStopTimeout
}
