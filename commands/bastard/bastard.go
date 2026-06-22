package bastard

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/golazy/lazy/commands"
)

const Prompt = `Analyze this GoLazy codebase and choose exactly three improvements to implement.

Constraints:
- Pick safe, focused changes with low blast radius.
- Prefer useful long-hanging fruit, cleanup that removes real friction, or small functionality that connects existing parts of the app or framework.
- Before editing, inspect the current worktree and avoid overwriting unrelated user changes.
- Implement the three improvements as three separate commits.
- After the commits are created, report exactly one commit per line using this format: <short-hash> <subject>.
- Then ask the user to take a break.`

type Command struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Help   func()

	Lookup func(string) (string, error)
	Runner commands.Runner
}

func (c Command) Execute(args []string) (int, error) {
	stdin := c.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	stdout := c.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := c.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	lookup := c.Lookup
	if lookup == nil {
		lookup = exec.LookPath
	}
	runner := c.Runner
	if runner == nil {
		runner = commands.Exec
	}

	flags := flag.NewFlagSet("bastard", flag.ContinueOnError)
	flags.SetOutput(stderr)
	forceCodex := flags.Bool("codex", false, "force Codex")
	forceClaude := flags.Bool("claude", false, "force Claude")
	if err := flags.Parse(args); err != nil {
		return 1, nil
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "lazy: usage: lazy bastard [--codex|--claude]")
		return 1, nil
	}
	if *forceCodex && *forceClaude {
		fmt.Fprintln(stderr, "lazy: use only one of --codex or --claude")
		return 1, nil
	}

	assistant, ok := c.selectAssistant(lookup, *forceCodex, *forceClaude)
	if !ok {
		if c.Help != nil {
			c.Help()
		}
		return 0, nil
	}

	if !confirm(stdin, stderr) {
		return 0, nil
	}

	if err := runner(assistant, []string{Prompt}, commands.Options{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}); err != nil {
		return 1, err
	}
	return 0, nil
}

func (c Command) selectAssistant(lookup func(string) (string, error), forceCodex bool, forceClaude bool) (string, bool) {
	if forceCodex {
		return lookupAssistant(lookup, "codex")
	}
	if forceClaude {
		return lookupAssistant(lookup, "claude")
	}
	if command, ok := lookupAssistant(lookup, "codex"); ok {
		return command, true
	}
	return lookupAssistant(lookup, "claude")
}

func lookupAssistant(lookup func(string) (string, error), name string) (string, bool) {
	path, err := lookup(name)
	if err != nil {
		return "", false
	}
	if path == "" {
		return name, true
	}
	return path, true
}

func confirm(stdin io.Reader, stderr io.Writer) bool {
	fmt.Fprintln(stderr, "Warning: the next step could consume credits or quota from your Claude or Codex subscription.")
	fmt.Fprintln(stderr, "GoLazy takes no responsibility for what happens on your machine.")
	fmt.Fprint(stderr, "Wanna take the risk? [y/N] ")

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		fmt.Fprintln(stderr)
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}
