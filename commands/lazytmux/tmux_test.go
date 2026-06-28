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
		Dir:        "/tmp/shop",
		PublicPath: "public_files",
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
		{command: "mise", args: []string{"exec", "--", "tmux", "split-window", "-d", "-t", "shop:dev", servicePreparedAppCommand([]lazyconfig.Service{{Name: "postgres"}}, "shop", "", "", "public_files")}, dir: "/tmp/shop"},
		{command: "mise", args: []string{"exec", "--", "tmux", "split-window", "-d", "-t", "shop:dev", "env LAZY_TMUX=1 LAZY_TMUX_SESSION=shop LAZY_MULTIVERSION=off lazy command-center"}, dir: "/tmp/shop"},
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

func TestServicePreparedAppCommand(t *testing.T) {
	got := servicePreparedAppCommand([]lazyconfig.Service{
		{Name: "postgres"},
		{Name: "minio"},
	}, "shop", "", "", "public_files")
	want := "set -e; lazy_service_task_exists() { mise tasks ls --all 2>/dev/null | awk '{print $1}' | grep -qx \"$1\"; }; lazy_service_wait_if_present() { task=\"$1:check\"; if lazy_service_task_exists \"$task\"; then until mise run \"$task\"; do sleep 1; done; fi; }; lazy_service_run_if_present() { if lazy_service_task_exists \"$1\"; then mise run \"$1\"; fi; }; lazy_service_wait_if_present postgres; lazy_service_run_if_present postgres:create; lazy_service_run_if_present postgres:migrate; lazy_service_wait_if_present minio; lazy_service_run_if_present minio:create; lazy_service_run_if_present minio:migrate; exec env LAZY_TMUX=1 LAZY_TMUX_SESSION=shop LAZY_MULTIVERSION=off lazy --publicpath public_files"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestServicePreparedAppCommandWithoutServices(t *testing.T) {
	got := servicePreparedAppCommand(nil, "shop", "", "", "public_files")
	want := "env LAZY_TMUX=1 LAZY_TMUX_SESSION=shop LAZY_MULTIVERSION=off lazy --publicpath public_files"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

type call struct {
	command string
	args    []string
	dir     string
}
