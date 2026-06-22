package lazytmux

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/golazy/lazy/commands"
	"github.com/golazy/lazy/commands/appcmd"
	"github.com/golazy/lazy/commands/lazyconfig"
)

const (
	InSessionEnv = "LAZY_TMUX"
	SessionEnv   = "LAZY_TMUX_SESSION"
)

type Command struct {
	Dir      string
	CmdPath  string
	ViewPath string
	Config   lazyconfig.Config
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	Runner   commands.Runner
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
		Command: appCommand(session, c.CmdPath, c.ViewPath),
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

func appCommand(session string, cmdPath string, viewPath string) string {
	parts := []string{
		"env",
		InSessionEnv + "=1",
		SessionEnv + "=" + shellQuote(session),
		"NO_VERSION_CHECK=true",
		"lazy",
	}
	if cmdPath != "" {
		parts = append(parts, "--cmdpath", shellQuote(filepath.ToSlash(cmdPath)))
	}
	if viewPath != "" && viewPath != appcmd.DefaultViewPath {
		parts = append(parts, "--viewpath", shellQuote(filepath.ToSlash(viewPath)))
	}
	return strings.Join(parts, " ")
}

func commandCenterCommand(session string) string {
	return strings.Join([]string{
		"env",
		InSessionEnv + "=1",
		SessionEnv + "=" + shellQuote(session),
		"NO_VERSION_CHECK=true",
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
