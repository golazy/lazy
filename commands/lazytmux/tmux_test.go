package lazytmux

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/lazyconfig"
)

func TestCommandBuildsTmuxSession(t *testing.T) {
	var calls []call
	runner := func(command string, args []string, options commands.Options) error {
		calls = append(calls, call{command: command, args: slices.Clone(args), dir: options.Dir})
		if slices.Equal(args, []string{"exec", "--", "tmux", "has-session", "-t", "shop"}) {
			return errors.New("missing session")
		}
		return nil
	}

	code, err := (Command{
		Dir: "/tmp/shop",
		Config: lazyconfig.Config{
			Tmux:     lazyconfig.Tmux{Session: "shop"},
			Services: []lazyconfig.Service{{Name: "postgres"}},
			Runners:  []lazyconfig.Process{{Name: "tailwind", Command: "lazy tailwind --watch"}},
			Programs: []lazyconfig.Program{{Name: "editor", Command: "nvim", Window: "work"}},
		},
		Runner: runner,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	want := []call{
		{command: "mise", args: []string{"exec", "--", "tmux", "has-session", "-t", "shop"}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "new-session", "-d", "-s", "shop", "-n", "dev", "mise run postgres:start"}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "split-window", "-d", "-t", "shop:dev", "lazy tailwind --watch"}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "split-window", "-d", "-t", "shop:dev", "env LAZY_TMUX=1 LAZY_TMUX_SESSION=shop NO_VERSION_CHECK=true lazy"}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "split-window", "-d", "-t", "shop:dev", "env LAZY_TMUX=1 LAZY_TMUX_SESSION=shop NO_VERSION_CHECK=true lazy command-center"}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "select-layout", "-t", "shop:dev", "tiled"}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "new-window", "-d", "-t", "shop:", "-n", "work", "nvim"}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "attach-session", "-t", "shop"}, dir: "/tmp/shop"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestCommandAttachesExistingTmuxSession(t *testing.T) {
	var calls []call
	runner := func(command string, args []string, options commands.Options) error {
		calls = append(calls, call{command: command, args: slices.Clone(args), dir: options.Dir})
		return nil
	}

	code, err := (Command{
		Dir:    "/tmp/shop",
		Config: lazyconfig.Config{Tmux: lazyconfig.Tmux{Session: "shop"}},
		Runner: runner,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	want := []call{
		{command: "mise", args: []string{"exec", "--", "tmux", "has-session", "-t", "shop"}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "attach-session", "-t", "shop"}, dir: "/tmp/shop"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

type call struct {
	command string
	args    []string
	dir     string
}
