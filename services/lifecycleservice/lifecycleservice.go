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

	"golazy.dev/lazy/services/configservice"
	"golazy.dev/lazy/services/taskservice"
)

const (
	defaultCheckInterval     = 500 * time.Millisecond
	defaultCheckWarningDelay = 5 * time.Second
	defaultStopTimeout       = 2 * time.Second
)

type TaskRunner func(ctx context.Context, dir string, task string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, capture bool) error

type Starter func(dir string, task string, stdout io.Writer, stderr io.Writer, stopTimeout time.Duration) (Process, error)

type TaskOutput struct {
	Service string
	Task    string
	Run     int
	Stream  string
}

type State string

const (
	StateStopped  State = "stopped"
	StateNotReady State = "not_ready"
	StateReady    State = "ready"
)

type Process interface {
	Done() <-chan error
	Stop()
	Kill()
}

type Service struct {
	Dir               string
	Config            configservice.Config
	Stdin             io.Reader
	Stdout            io.Writer
	Stderr            io.Writer
	Runner            TaskRunner
	Starter           Starter
	Register          func([]string)
	Status            func(service string, state State, message string)
	Output            func(TaskOutput) io.Writer
	CheckInterval     time.Duration
	CheckWarningDelay time.Duration
	StopTimeout       time.Duration
}

type Manager struct {
	services          []string
	ready             chan error
	service           Service
	inventory         taskservice.Inventory
	processesMu       sync.Mutex
	processes         map[string]Process
	stoppingProcesses map[Process]struct{}
	stopping          bool
	runsMu            sync.Mutex
	runs              map[string]int
}

func (s Service) Start(ctx context.Context) (*Manager, error) {
	dir := s.Dir
	if dir == "" {
		dir = "."
	}
	inventory, err := taskservice.Inspect(dir, s.Config)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		services:          append([]string(nil), inventory.Services...),
		ready:             make(chan error, 1),
		service:           s,
		inventory:         inventory,
		processes:         map[string]Process{},
		stoppingProcesses: map[Process]struct{}{},
		runs:              map[string]int{},
	}
	if len(inventory.Services) == 0 {
		manager.ready <- nil
		return manager, nil
	}
	s.register(inventory.Services)

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

func (m *Manager) Restart(ctx context.Context, service string) error {
	if m == nil {
		return fmt.Errorf("service manager is not running")
	}
	service = strings.TrimSpace(service)
	if service == "" {
		return fmt.Errorf("service name is required")
	}
	if !m.hasService(service) {
		return fmt.Errorf("unknown service %q", service)
	}

	process := m.takeProcessForRestart(service)
	if process != nil {
		m.service.status(service, StateNotReady, "Restarting service.")
		m.service.logf(service, "", 0, "lazy: restarting %s service\n", service)
		process.Stop()
	}
	return m.service.startService(ctx, m, m.inventory, service)
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

func (m *Manager) takeProcessForRestart(name string) Process {
	m.processesMu.Lock()
	defer m.processesMu.Unlock()
	process := m.processes[name]
	if process == nil {
		return nil
	}
	delete(m.processes, name)
	m.stoppingProcesses[process] = struct{}{}
	return process
}

func (m *Manager) removeProcess(name string, process Process) {
	m.processesMu.Lock()
	defer m.processesMu.Unlock()
	if m.processes[name] == process {
		delete(m.processes, name)
	}
}

func (m *Manager) ignoreProcessExit(process Process) bool {
	m.processesMu.Lock()
	defer m.processesMu.Unlock()
	if m.stopping {
		return true
	}
	if _, ok := m.stoppingProcesses[process]; ok {
		delete(m.stoppingProcesses, process)
		return true
	}
	return false
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

func (m *Manager) hasService(service string) bool {
	for _, known := range m.services {
		if known == service {
			return true
		}
	}
	return false
}

func (m *Manager) nextRun(service string, task string) int {
	m.runsMu.Lock()
	defer m.runsMu.Unlock()
	key := service + "\x00" + task
	m.runs[key]++
	return m.runs[key]
}

func (s Service) startService(ctx context.Context, manager *Manager, inventory taskservice.Inventory, service string) error {
	startTask := taskservice.TaskName(service, "start")
	startRun := manager.nextRun(service, "start")
	s.status(service, StateNotReady, "Starting service.")
	s.logf(service, "start", startRun, "lazy: starting %s service\n", service)
	process, err := s.starter()(s.dir(), startTask, s.output(service, "start", startRun, "stdout"), s.output(service, "start", startRun, "stderr"), s.stopTimeout())
	if err != nil {
		s.status(service, StateStopped, err.Error())
		return fmt.Errorf("%s:start: %w", service, err)
	}
	if !manager.addProcess(service, process) {
		process.Stop()
		return context.Canceled
	}

	if taskservice.HasTask(inventory.Tasks, service, "check") {
		if err := s.waitForCheck(ctx, manager, service, process); err != nil {
			return err
		}
	} else {
		s.status(service, StateReady, "Service started.")
	}

	for _, action := range []string{"create", "migrate"} {
		if !taskservice.HasTask(inventory.Tasks, service, action) {
			continue
		}
		task := taskservice.TaskName(service, action)
		run := manager.nextRun(service, action)
		s.logf(service, action, run, "lazy: running %s\n", task)
		if err := s.runner()(ctx, s.dir(), task, nil, s.Stdin, s.output(service, action, run, "stdout"), s.output(service, action, run, "stderr"), false); err != nil {
			s.logf(service, action, run, "lazy: %s failed: %v\n", task, err)
			continue
		}
		s.logf(service, action, run, "lazy: %s finished\n", task)
	}

	go s.watchProcess(manager, service, process)
	return nil
}

func (s Service) waitForCheck(ctx context.Context, manager *Manager, service string, process Process) error {
	task := taskservice.TaskName(service, "check")
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
		run := manager.nextRun(service, "check")
		err := s.runner()(ctx, s.dir(), task, nil, s.Stdin, s.output(service, "check", run, "stdout"), s.output(service, "check", run, "stderr"), false)
		if err == nil {
			s.status(service, StateReady, "Service is ready.")
			s.logf(service, "check", run, "lazy: %s is ready\n", service)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case processErr := <-process.Done():
			manager.removeProcess(service, process)
			if processErr == nil {
				s.status(service, StateStopped, fmt.Sprintf("%s exited before readiness.", startName(service)))
				return fmt.Errorf("%s:start exited before %s succeeded", service, task)
			}
			s.status(service, StateStopped, processErr.Error())
			return fmt.Errorf("%s:start exited before %s succeeded: %w", service, task, processErr)
		case <-warning.C:
			if !warned {
				s.logf(service, "check", run, "lazy: %s is still failing after %s; check the service output. lazy will keep checking.\n", task, warningDelay.Round(time.Millisecond))
				if message := strings.TrimSpace(err.Error()); message != "" {
					s.logf(service, "check", run, "lazy: latest %s error: %s\n", task, message)
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
	if manager.ignoreProcessExit(process) {
		return
	}
	if err != nil {
		s.status(service, StateStopped, err.Error())
		s.logf(service, "", 0, "lazy: %s service exited: %v\n", service, err)
		return
	}
	s.status(service, StateStopped, "Service stopped.")
	s.logf(service, "", 0, "lazy: %s service exited\n", service)
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

func (s Service) register(services []string) {
	if s.Register != nil {
		s.Register(append([]string(nil), services...))
	}
	for _, service := range services {
		s.status(service, StateNotReady, "Waiting to start.")
	}
}

func (s Service) status(service string, state State, message string) {
	if s.Status != nil {
		s.Status(service, state, message)
	}
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

func (s Service) output(service string, task string, run int, stream string) io.Writer {
	if s.Output != nil {
		return s.Output(TaskOutput{Service: service, Task: task, Run: run, Stream: stream})
	}
	if stream == "stderr" {
		return s.stderr()
	}
	return s.stdout()
}

func (s Service) logf(service string, task string, run int, format string, args ...any) {
	_, _ = fmt.Fprintf(s.output(service, task, run, "stderr"), format, args...)
}

func startName(service string) string {
	return taskservice.TaskName(service, "start")
}

func runTask(ctx context.Context, dir string, task string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, capture bool) error {
	command, runArgs, env := taskservice.TaskCommand(task, args)
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
	command, args, env := taskservice.TaskCommand(task, nil)
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
