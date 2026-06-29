package buildservice

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golazy.dev/lazy/services/appservice"
	"golazy.dev/lazy/services/workspaceservice"
)

const defaultStopTimeout = 2 * time.Second

type State string

const (
	StateQueued       State = "queued"
	StateBuilding     State = "building"
	StateBuildFailed  State = "build_failed"
	StateStarting     State = "starting"
	StateRunning      State = "running"
	StateRunFailed    State = "run_failed"
	StateReloadFailed State = "reload_failed"
	StateStopped      State = "stopped"
)

type EventType string

const (
	EventState      EventType = "state"
	EventOutput     EventType = "output"
	EventFileChange EventType = "file_change"
	EventReload     EventType = "reload"
	EventManual     EventType = "manual"
)

type ServiceState string

const (
	ServiceStopped  ServiceState = "stopped"
	ServiceNotReady ServiceState = "not_ready"
	ServiceReady    ServiceState = "ready"
)

type Event struct {
	Type      EventType `json:"type"`
	Time      time.Time `json:"time"`
	State     State     `json:"state,omitempty"`
	Message   string    `json:"message,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Service   string    `json:"service,omitempty"`
	Task      string    `json:"task,omitempty"`
	Run       int       `json:"run,omitempty"`
	Output    string    `json:"output,omitempty"`
	Changed   []string  `json:"changed,omitempty"`
	Build     int       `json:"build,omitempty"`
	Duration  string    `json:"duration,omitempty"`
	AppAddr   string    `json:"app_addr,omitempty"`
	PanelAddr string    `json:"panel_addr,omitempty"`
}

type ServiceSnapshot struct {
	Name    string       `json:"name"`
	State   ServiceState `json:"state"`
	Message string       `json:"message,omitempty"`
}

type Action string

const (
	ActionRebuild        Action = "rebuild"
	ActionRestart        Action = "restart"
	ActionRestartService Action = "restart_service"
)

type ActionRequest struct {
	Action  Action
	Service string
	Reply   chan error
}

type Actions chan ActionRequest

type storeContextKey struct{}
type actionsContextKey struct{}

func NewActions() Actions {
	return make(chan ActionRequest, 8)
}

func WithStore(ctx context.Context, store *Store) context.Context {
	if store == nil {
		return ctx
	}
	return context.WithValue(ctx, storeContextKey{}, store)
}

func StoreFromContext(ctx context.Context) (*Store, bool) {
	store, ok := ctx.Value(storeContextKey{}).(*Store)
	return store, ok && store != nil
}

func WithActions(ctx context.Context, actions Actions) context.Context {
	if actions == nil {
		return ctx
	}
	return context.WithValue(ctx, actionsContextKey{}, actions)
}

func ActionsFromContext(ctx context.Context) (Actions, bool) {
	actions, ok := ctx.Value(actionsContextKey{}).(Actions)
	return actions, ok && actions != nil
}

func (a Actions) Enqueue(ctx context.Context, action Action) error {
	return a.EnqueueService(ctx, action, "")
}

func (a Actions) EnqueueService(ctx context.Context, action Action, service string) error {
	request := ActionRequest{
		Action:  action,
		Service: service,
		Reply:   make(chan error, 1),
	}
	select {
	case a <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.Reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type Snapshot struct {
	State            State             `json:"state"`
	Message          string            `json:"message"`
	CommandPath      string            `json:"command_path"`
	WatchedRoot      string            `json:"watched_root"`
	BuildCount       int               `json:"build_count"`
	Duration         string            `json:"duration,omitempty"`
	StartedAt        time.Time         `json:"started_at,omitempty"`
	AppAddr          string            `json:"app_addr,omitempty"`
	ControlPlaneAddr string            `json:"control_plane_addr,omitempty"`
	Changed          []string          `json:"changed,omitempty"`
	Output           string            `json:"output,omitempty"`
	Services         []ServiceSnapshot `json:"services,omitempty"`
	Events           []Event           `json:"events,omitempty"`
}

type Store struct {
	mu        sync.RWMutex
	snapshot  Snapshot
	events    []Event
	limit     int
	listeners map[chan Event]struct{}
}

func NewStore(limit int) *Store {
	if limit <= 0 {
		limit = 200
	}
	return &Store{
		limit:     limit,
		listeners: map[chan Event]struct{}{},
	}
}

func (s *Store) Update(snapshot Snapshot) {
	s.record(snapshot, Event{Type: EventState, State: snapshot.State, Message: snapshot.Message, Build: snapshot.BuildCount})
}

func (s *Store) AddEvent(event Event) {
	s.recordEvent(event)
}

func (s *Store) SetServices(names []string) {
	snapshot := s.Snapshot()
	services := make([]ServiceSnapshot, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		services = append(services, ServiceSnapshot{Name: name, State: ServiceNotReady})
	}
	snapshot.Services = services
	s.record(snapshot, Event{Type: EventManual, Message: "Development services discovered."})
}

func (s *Store) UpdateService(name string, state ServiceState, message string) {
	if name == "" {
		return
	}
	snapshot := s.Snapshot()
	found := false
	for index := range snapshot.Services {
		if snapshot.Services[index].Name != name {
			continue
		}
		snapshot.Services[index].State = state
		snapshot.Services[index].Message = message
		found = true
		break
	}
	if !found {
		snapshot.Services = append(snapshot.Services, ServiceSnapshot{Name: name, State: state, Message: message})
	}
	s.record(snapshot, Event{Type: EventManual, Service: name, Message: message})
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := s.snapshot
	snapshot.Changed = append([]string(nil), snapshot.Changed...)
	snapshot.Services = append([]ServiceSnapshot(nil), snapshot.Services...)
	snapshot.Events = append([]Event(nil), s.events...)
	return snapshot
}

func (s *Store) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 32)
	s.mu.Lock()
	s.listeners[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.listeners, ch)
		s.mu.Unlock()
	}
}

func (s *Store) record(snapshot Snapshot, event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	s.mu.Lock()
	if snapshot.Services == nil {
		snapshot.Services = append([]ServiceSnapshot(nil), s.snapshot.Services...)
	}
	s.snapshot = snapshot
	s.events = append(s.events, event)
	if len(s.events) > s.limit {
		s.events = append([]Event(nil), s.events[len(s.events)-s.limit:]...)
	}
	listeners := make([]chan Event, 0, len(s.listeners))
	for ch := range s.listeners {
		listeners = append(listeners, ch)
	}
	s.mu.Unlock()

	for _, ch := range listeners {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Store) recordEvent(event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	s.mu.Lock()
	s.events = append(s.events, event)
	if len(s.events) > s.limit {
		s.events = append([]Event(nil), s.events[len(s.events)-s.limit:]...)
	}
	listeners := make([]chan Event, 0, len(s.listeners))
	for ch := range s.listeners {
		listeners = append(listeners, ch)
	}
	s.mu.Unlock()

	for _, ch := range listeners {
		select {
		case ch <- event:
		default:
		}
	}
}

type Config struct {
	Root           string
	CommandPath    string
	ViewPath       string
	PublicPath     string
	GoWork         string
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	StartupTimeout time.Duration
	StopTimeout    time.Duration
}

type BuildResult struct {
	Binary   string
	Output   string
	Err      error
	Duration time.Duration
}

type Process struct {
	command          *exec.Cmd
	addr             string
	controlPlaneAddr string
	done             chan error
	stopTimeout      time.Duration
}

func (c Config) Build(ctx context.Context, tmpDir string, buildNumber int) BuildResult {
	started := time.Now()
	binary := filepath.Join(tmpDir, "app-"+strconv.Itoa(buildNumber)+exeSuffix())

	var output bytes.Buffer
	workspaceActive, err := workspaceservice.Active(c.Root, c.GoWork)
	if err != nil {
		err = fmt.Errorf("inspect Go workspace: %w", err)
	} else if !workspaceActive {
		tidy := exec.CommandContext(ctx, "go", "mod", "tidy")
		tidy.Dir = c.Root
		tidy.Stdout = &output
		tidy.Stderr = &output
		err = tidy.Run()
	}
	if err == nil {
		var buildFlags []string
		buildFlags, err = appservice.LazyDevBuildFlags(c.Root, c.ViewPath, c.PublicPath)
		if err == nil {
			args := appservice.GoBuildArgs("lazydev", filepath.ToSlash(c.CommandPath), binary, buildFlags...)
			build := exec.CommandContext(ctx, "go", args...)
			build.Dir = c.Root
			build.Stdout = &output
			build.Stderr = &output
			err = build.Run()
		}
	}
	return BuildResult{
		Binary:   binary,
		Output:   output.String(),
		Err:      err,
		Duration: time.Since(started),
	}
}

func (c Config) Start(ctx context.Context, binary string) (*Process, error) {
	addr, err := freeLoopbackAddr()
	if err != nil {
		return nil, err
	}
	controlPlaneAddr, err := freeLoopbackAddr()
	if err != nil {
		return nil, err
	}
	stopTimeout := stopTimeoutOrDefault(c.StopTimeout)
	cmd := exec.CommandContext(ctx, binary)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := cmd.Process.Signal(os.Interrupt)
		if errors.Is(err, os.ErrProcessDone) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = stopTimeout
	cmd.Dir = c.Root
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Env = developmentAppEnv(os.Environ(), addr, controlPlaneAddr)

	done := make(chan error, 1)
	process := &Process{
		command:          cmd,
		addr:             addr,
		controlPlaneAddr: controlPlaneAddr,
		done:             done,
		stopTimeout:      stopTimeout,
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		done <- cmd.Wait()
		close(done)
	}()

	if err := waitForTCP(ctx, addr, done, c.StartupTimeout); err != nil {
		process.Stop()
		return nil, err
	}
	if err := waitForTCP(ctx, controlPlaneAddr, done, c.StartupTimeout); err != nil {
		process.Stop()
		return nil, err
	}
	return process, nil
}

func (p *Process) Addr() string {
	if p == nil {
		return ""
	}
	return p.addr
}

func (p *Process) ControlPlaneAddr() string {
	if p == nil {
		return ""
	}
	return p.controlPlaneAddr
}

func (p *Process) Done() <-chan error {
	if p == nil {
		return nil
	}
	return p.done
}

func (p *Process) Stop() {
	if p == nil || p.command == nil || p.command.Process == nil {
		return
	}
	stopTimeout := stopTimeoutOrDefault(p.stopTimeout)
	_ = p.command.Process.Signal(os.Interrupt)
	select {
	case <-p.done:
		return
	case <-time.After(stopTimeout):
		_ = p.command.Process.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(stopTimeout):
	}
}

func (p *Process) Kill() {
	if p == nil || p.command == nil || p.command.Process == nil {
		return
	}
	_ = p.command.Process.Kill()
}

func waitForTCP(ctx context.Context, addr string, done <-chan error, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			if err == nil {
				return errors.New("application exited before accepting connections")
			}
			return fmt.Errorf("application exited before accepting connections: %w", err)
		case <-deadline.C:
			return fmt.Errorf("application did not listen on %s within %s", addr, timeout)
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

func freeLoopbackAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("find free loopback port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().String(), nil
}

func stopTimeoutOrDefault(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	return defaultStopTimeout
}

func developmentAppEnv(base []string, addr, controlPlaneAddr string) []string {
	env := append([]string(nil), base...)
	for _, value := range []struct {
		key   string
		value string
	}{
		{key: "ADDR", value: addr},
		{key: "CONTROL_PLANE_ADDR", value: controlPlaneAddr},
	} {
		env = setEnvValue(env, value.key, value.value)
	}
	return env
}

func setEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	next := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				next = append(next, prefix+value)
				replaced = true
			}
			continue
		}
		next = append(next, entry)
	}
	if !replaced {
		next = append(next, prefix+value)
	}
	return next
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
