package terminalservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"golazy.dev/lazytui/encoding/tty"
	"golazy.dev/lazytui/pty"
	"golazy.dev/lazywebsocket"
)

type contextKey struct{}

type Manager struct {
	Shell func() string
	Env   func() []string

	mu      sync.Mutex
	session *Session
}

type Options struct {
	Dir  string
	Size tty.Size
}

func NewManager() *Manager {
	return &Manager{
		Shell: defaultShell,
		Env:   os.Environ,
	}
}

func WithManager(ctx context.Context, manager *Manager) context.Context {
	if manager == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, manager)
}

func ManagerFromContext(ctx context.Context) (*Manager, bool) {
	manager, ok := ctx.Value(contextKey{}).(*Manager)
	return manager, ok && manager != nil
}

func (m *Manager) Attach(w http.ResponseWriter, r *http.Request, opts Options) error {
	session, err := m.Session(r.Context(), opts)
	if err != nil {
		return err
	}
	return session.Attach(w, r)
}

func (m *Manager) Session(ctx context.Context, opts Options) (*Session, error) {
	if opts.Size == (tty.Size{}) {
		opts.Size = tty.Size{Rows: 24, Cols: 80}
	}
	if !opts.Size.Valid() {
		opts.Size = tty.Size{Rows: 24, Cols: 80}
	}
	dir, err := cleanDir(opts.Dir)
	if err != nil {
		return nil, err
	}
	session, err := m.sessionFor(ctx, dir, opts.Size)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session == nil {
		return Snapshot{State: StateIdle, Supported: true}
	}
	return m.session.Snapshot()
}

func (m *Manager) sessionFor(ctx context.Context, dir string, size tty.Size) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil && m.session.Running() && m.session.Dir() == dir {
		return m.session, nil
	}
	if m.session != nil {
		m.session.Close()
	}
	shell := ""
	if m.Shell != nil {
		shell = m.Shell()
	}
	if shell == "" {
		shell = defaultShell()
	}
	env := os.Environ()
	if m.Env != nil {
		env = m.Env()
	}
	session := NewSession(SessionConfig{
		Shell: shell,
		Dir:   dir,
		Env:   terminalEnv(env),
		Size:  size,
	})
	if err := session.Start(ctx); err != nil {
		return nil, err
	}
	m.session = session
	return session, nil
}

func cleanDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("terminal working directory is not available")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve terminal working directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect terminal working directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("terminal working directory is not a directory")
	}
	return abs, nil
}

func terminalEnv(env []string) []string {
	out := append([]string(nil), env...)
	hasTerm := false
	for _, value := range out {
		if len(value) >= len("TERM=") && value[:len("TERM=")] == "TERM=" {
			hasTerm = true
			break
		}
	}
	if !hasTerm {
		out = append(out, "TERM=xterm-256color")
	}
	return out
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

type State string

const (
	StateIdle     State = "idle"
	StateRunning  State = "running"
	StateExited   State = "exited"
	StateErrored  State = "errored"
	StateDetached State = "detached"
)

type Snapshot struct {
	State     State    `json:"state"`
	Dir       string   `json:"dir,omitempty"`
	Shell     string   `json:"shell,omitempty"`
	Size      tty.Size `json:"size,omitempty"`
	Error     string   `json:"error,omitempty"`
	Clients   int      `json:"clients,omitempty"`
	Started   bool     `json:"started"`
	Supported bool     `json:"supported"`
}

type SessionConfig struct {
	Shell string
	Dir   string
	Env   []string
	Size  tty.Size
}

type Session struct {
	config SessionConfig

	ctx    context.Context
	cancel context.CancelFunc
	cmd    *pty.Cmd

	mu      sync.Mutex
	inputMu sync.Mutex
	state   State
	err     error
	clients map[chan frame]struct{}
	once    sync.Once
}

type frame struct {
	messageType int
	data        []byte
}

func NewSession(config SessionConfig) *Session {
	if !config.Size.Valid() {
		config.Size = tty.Size{Rows: 24, Cols: 80}
	}
	return &Session{
		config:  config,
		state:   StateIdle,
		clients: map[chan frame]struct{}{},
	}
}

func (s *Session) Start(ctx context.Context) error {
	if s == nil {
		return errors.New("terminal session is nil")
	}
	if s.config.Shell == "" {
		return fmt.Errorf("terminal shell is not configured")
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	cmd := pty.CommandContext(sessionCtx, s.config.Shell)
	cmd.Dir = s.config.Dir
	cmd.Env = s.config.Env
	cmd.Size = s.config.Size
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start terminal shell: %w", err)
	}
	s.ctx = sessionCtx
	s.cancel = cancel
	s.cmd = cmd
	s.setState(StateRunning, nil)
	go s.readOutput(cmd)
	go s.wait(cmd)
	return nil
}

func (s *Session) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == StateRunning
}

func (s *Session) Dir() string {
	if s == nil {
		return ""
	}
	return s.config.Dir
}

func (s *Session) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := Snapshot{
		State:     s.state,
		Dir:       s.config.Dir,
		Shell:     s.config.Shell,
		Size:      s.config.Size,
		Clients:   len(s.clients),
		Started:   s.cmd != nil,
		Supported: true,
	}
	if s.err != nil {
		snapshot.Error = s.err.Error()
	}
	return snapshot
}

func (s *Session) Attach(w http.ResponseWriter, r *http.Request) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch := s.register()
	defer s.unregister(ch)
	if err := conn.WriteJSON(controlMessage{Type: "status", State: string(StateRunning)}); err != nil {
		return err
	}

	writeDone := make(chan error, 1)
	go func() {
		defer conn.Close()
		for item := range ch {
			if err := conn.WriteMessage(item.messageType, item.data); err != nil {
				writeDone <- err
				return
			}
		}
		writeDone <- nil
	}()

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			return nil
		}
		select {
		case err := <-writeDone:
			return err
		default:
		}
		switch messageType {
		case lazywebsocket.BinaryMessage:
			if err := s.Write(data); err != nil {
				return conn.WriteJSON(controlMessage{Type: "error", Error: err.Error()})
			}
		case lazywebsocket.TextMessage:
			if err := s.handleControl(data); err != nil {
				return conn.WriteJSON(controlMessage{Type: "error", Error: err.Error()})
			}
		}
	}
}

func (s *Session) Write(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil || cmd.Master() == nil {
		return pty.ErrNotStarted
	}
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	_, err := cmd.Master().Write(data)
	return err
}

func (s *Session) Resize(size tty.Size) error {
	if !size.Valid() {
		return tty.ErrInvalidSize
	}
	s.mu.Lock()
	cmd := s.cmd
	s.config.Size = size
	s.mu.Unlock()
	if cmd == nil {
		return pty.ErrNotStarted
	}
	return cmd.Resize(size)
}

func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.finish(StateDetached, nil)
}

func (s *Session) readOutput(cmd *pty.Cmd) {
	master := cmd.Master()
	if master == nil {
		s.finish(StateErrored, pty.ErrNotStarted)
		return
	}
	buf := make([]byte, 4096)
	for {
		n, err := master.Read(buf)
		if n > 0 {
			s.broadcast(frame{messageType: lazywebsocket.BinaryMessage, data: append([]byte(nil), buf[:n]...)})
		}
		if err != nil {
			return
		}
	}
}

func (s *Session) wait(cmd *pty.Cmd) {
	err := cmd.Wait()
	if err != nil && s.ctx != nil && s.ctx.Err() != nil {
		err = nil
	}
	if err != nil {
		s.finish(StateErrored, err)
		return
	}
	s.finish(StateExited, nil)
}

func (s *Session) handleControl(data []byte) error {
	var message controlMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return fmt.Errorf("parse terminal control message: %w", err)
	}
	switch message.Type {
	case "resize":
		return s.Resize(tty.Size{Rows: message.Rows, Cols: message.Cols})
	default:
		return fmt.Errorf("unknown terminal control message %q", message.Type)
	}
}

func (s *Session) register() chan frame {
	ch := make(chan frame, 128)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func (s *Session) unregister(ch chan frame) {
	s.mu.Lock()
	if _, ok := s.clients[ch]; ok {
		delete(s.clients, ch)
		close(ch)
	}
	s.mu.Unlock()
}

func (s *Session) broadcast(item frame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- item:
		default:
		}
	}
}

func (s *Session) setState(state State, err error) {
	s.mu.Lock()
	s.state = state
	s.err = err
	s.mu.Unlock()
}

func (s *Session) finish(state State, err error) {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.setState(state, err)
		status := controlMessage{Type: "status", State: string(state)}
		if err != nil {
			status.Error = err.Error()
		}
		data, _ := json.Marshal(status)
		s.mu.Lock()
		for ch := range s.clients {
			select {
			case ch <- frame{messageType: lazywebsocket.TextMessage, data: data}:
			default:
			}
			close(ch)
			delete(s.clients, ch)
		}
		s.mu.Unlock()
	})
}

type controlMessage struct {
	Type  string `json:"type"`
	State string `json:"state,omitempty"`
	Error string `json:"error,omitempty"`
	Rows  int    `json:"rows,omitempty"`
	Cols  int    `json:"cols,omitempty"`
}

var upgrader = lazywebsocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
}
