package bastard

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/golazy/lazy/commands"
)

func TestCommandShowsHelpWhenNoAssistantIsAvailable(t *testing.T) {
	var helpCalled bool
	var stderr bytes.Buffer

	code, err := (Command{
		Stdin:  strings.NewReader("yes\n"),
		Stderr: &stderr,
		Help: func() {
			helpCalled = true
		},
		Lookup: missingLookup,
		Runner: func(string, []string, commands.Options) error {
			t.Fatalf("runner should not be called")
			return nil
		},
	}).Execute(nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !helpCalled {
		t.Fatalf("help was not called")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCommandStopsWhenUserDeclines(t *testing.T) {
	var stderr bytes.Buffer

	code, err := (Command{
		Stdin:  strings.NewReader("\n"),
		Stderr: &stderr,
		Lookup: func(name string) (string, error) {
			if name == "codex" {
				return "/bin/codex", nil
			}
			return "", errors.New("missing")
		},
		Runner: func(string, []string, commands.Options) error {
			t.Fatalf("runner should not be called")
			return nil
		},
	}).Execute(nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Wanna take the risk? [y/N]") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCommandLaunchesCodexByDefault(t *testing.T) {
	var call runnerCall

	code, err := (Command{
		Stdin:  strings.NewReader("yes\n"),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Lookup: func(name string) (string, error) {
			switch name {
			case "codex":
				return "/bin/codex", nil
			case "claude":
				return "/bin/claude", nil
			default:
				return "", errors.New("missing")
			}
		},
		Runner: captureRunner(&call),
	}).Execute(nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if call.command != "/bin/codex" {
		t.Fatalf("command = %q, want /bin/codex", call.command)
	}
	if len(call.args) != 1 || !strings.Contains(call.args[0], "exactly three improvements") {
		t.Fatalf("args = %#v", call.args)
	}
	if !strings.Contains(call.args[0], "three separate commits") {
		t.Fatalf("prompt = %q", call.args[0])
	}
}

func TestCommandCanForceClaude(t *testing.T) {
	var call runnerCall

	code, err := (Command{
		Stdin:  strings.NewReader("y\n"),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Lookup: func(name string) (string, error) {
			if name == "claude" {
				return "/bin/claude", nil
			}
			return "", errors.New("missing")
		},
		Runner: captureRunner(&call),
	}).Execute([]string{"--claude"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if call.command != "/bin/claude" {
		t.Fatalf("command = %q, want /bin/claude", call.command)
	}
}

func TestCommandRejectsBothAssistantFlags(t *testing.T) {
	var stderr bytes.Buffer

	code, err := (Command{
		Stderr: &stderr,
	}).Execute([]string{"--codex", "--claude"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "use only one") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCommandRejectsArguments(t *testing.T) {
	var stderr bytes.Buffer

	code, err := (Command{
		Stderr: &stderr,
	}).Execute([]string{"extra"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lazy bastard") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

type runnerCall struct {
	command string
	args    []string
}

func captureRunner(call *runnerCall) commands.Runner {
	return func(command string, args []string, options commands.Options) error {
		call.command = command
		call.args = slices.Clone(args)
		return nil
	}
}

func missingLookup(string) (string, error) {
	return "", errors.New("missing")
}
