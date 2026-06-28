package lazytmux

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/appcmd"
	"golazy.dev/lazy/commands/lazyconfig"
)

const (
	InSessionEnv = "LAZY_TMUX"
	SessionEnv   = "LAZY_TMUX_SESSION"
)

type Command struct {
	Dir        string
	CmdPath    string
	ViewPath   string
	PublicPath string
	Config     lazyconfig.Config
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Runner     commands.Runner
}

func (c Command) Execute() (int, error) {
	runner := c.Runner
	if runner == nil {
		runner = commands.Exec
	}

	session := c.Config.Tmux.Session
	if session == "" {
		session = defaultSessionName(c.Dir)
	}

	panes := c.panes(session)
	if len(panes) == 0 {
		panes = []pane{{Title: "command-center", Command: commandCenterCommand(session)}}
	}

	if err := c.tmuxQuiet(runner, "has-session", "-t", session); err == nil {
		if err := c.tmuxAttach(runner, session); err != nil {
			return 1, fmt.Errorf("attach tmux session: %w", err)
		}
		return 0, nil
	}

	if err := c.tmux(runner, "new-session", "-d", "-s", session, "-n", "dev", panes[0].Command); err != nil {
		return 1, fmt.Errorf("start tmux session: %w", err)
	}
	for _, pane := range panes[1:] {
		if err := c.tmux(runner, "split-window", "-d", "-t", session+":dev", pane.Command); err != nil {
			return 1, fmt.Errorf("create %s pane: %w", pane.Title, err)
		}
	}
	if len(panes) > 1 {
		if err := c.tmux(runner, "select-layout", "-t", session+":dev", "tiled"); err != nil {
			return 1, fmt.Errorf("select tmux layout: %w", err)
		}
	}
	for _, program := range c.Config.Programs {
		window := program.Window
		if window == "" {
			window = program.Name
		}
		if err := c.tmux(runner, "new-window", "-d", "-t", session+":", "-n", window, program.Command); err != nil {
			return 1, fmt.Errorf("create %s window: %w", program.Name, err)
		}
	}
	if err := c.tmuxAttach(runner, session); err != nil {
		return 1, fmt.Errorf("attach tmux session: %w", err)
	}
	return 0, nil
}

type pane struct {
	Title   string
	Command string
}

func (c Command) panes(session string) []pane {
	var panes []pane
	for _, service := range c.Config.Services {
		panes = append(panes, pane{
			Title:   service.Name,
			Command: "mise run " + shellQuote(service.Name+":start"),
		})
	}
	for _, runner := range c.Config.Runners {
		panes = append(panes, pane{
			Title:   runner.Name,
			Command: runner.Command,
		})
	}
	panes = append(panes, pane{
		Title:   "app",
		Command: servicePreparedAppCommand(c.Config.Services, session, c.CmdPath, c.ViewPath, c.PublicPath),
	})
	panes = append(panes, pane{
		Title:   "command-center",
		Command: commandCenterCommand(session),
	})
	return panes
}

func (c Command) tmux(runner commands.Runner, args ...string) error {
	return runner("mise", append([]string{"exec", "--", "tmux"}, args...), commands.Options{
		Dir:    c.Dir,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	})
}

func (c Command) tmuxQuiet(runner commands.Runner, args ...string) error {
	return runner("mise", append([]string{"exec", "--", "tmux"}, args...), commands.Options{
		Dir:     c.Dir,
		Stdout:  io.Discard,
		Stderr:  io.Discard,
		Capture: true,
	})
}

func (c Command) tmuxAttach(runner commands.Runner, session string) error {
	return runner("mise", []string{"exec", "--", "tmux", "attach-session", "-t", session}, commands.Options{
		Dir:    c.Dir,
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	})
}

func appCommand(session string, cmdPath string, viewPath string, publicPath string) string {
	parts := []string{
		"env",
		InSessionEnv + "=1",
		SessionEnv + "=" + shellQuote(session),
		"LAZY_MULTIVERSION=off",
		"lazy",
	}
	if cmdPath != "" {
		parts = append(parts, "--cmdpath", shellQuote(filepath.ToSlash(cmdPath)))
	}
	if viewPath != "" && viewPath != appcmd.DefaultViewPath {
		parts = append(parts, "--viewpath", shellQuote(filepath.ToSlash(viewPath)))
	}
	if publicPath != "" && publicPath != appcmd.DefaultPublicPath {
		parts = append(parts, "--publicpath", shellQuote(filepath.ToSlash(publicPath)))
	}
	return strings.Join(parts, " ")
}

func servicePreparedAppCommand(services []lazyconfig.Service, session string, cmdPath string, viewPath string, publicPath string) string {
	app := appCommand(session, cmdPath, viewPath, publicPath)
	setup := serviceSetupCommand(services)
	if setup == "" {
		return app
	}
	return setup + "; exec " + app
}

func serviceSetupCommand(services []lazyconfig.Service) string {
	if len(services) == 0 {
		return ""
	}
	parts := []string{
		"set -e",
		"lazy_service_task_exists() { mise tasks ls --all 2>/dev/null | awk '{print $1}' | grep -qx \"$1\"; }",
		"lazy_service_wait_if_present() { task=\"$1:check\"; if lazy_service_task_exists \"$task\"; then until mise run \"$task\"; do sleep 1; done; fi; }",
		"lazy_service_run_if_present() { if lazy_service_task_exists \"$1\"; then mise run \"$1\"; fi; }",
	}
	for _, service := range services {
		name := shellQuote(service.Name)
		parts = append(parts,
			"lazy_service_wait_if_present "+name,
			"lazy_service_run_if_present "+shellQuote(service.Name+":create"),
			"lazy_service_run_if_present "+shellQuote(service.Name+":migrate"),
		)
	}
	return strings.Join(parts, "; ")
}

func commandCenterCommand(session string) string {
	return strings.Join([]string{
		"env",
		InSessionEnv + "=1",
		SessionEnv + "=" + shellQuote(session),
		"LAZY_MULTIVERSION=off",
		"lazy",
		"command-center",
	}, " ")
}

func defaultSessionName(dir string) string {
	if dir == "" {
		if workingDir, err := os.Getwd(); err == nil {
			dir = workingDir
		}
	}
	name := filepath.Base(dir)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "app"
	}
	name = regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "app"
	}
	return "lazy-" + name
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`).MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
