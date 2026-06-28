package mise

import (
	"errors"
	"reflect"
	"testing"

	"golazy.dev/lazy/commands"
)

func TestDetectNodePackageManagerPrefersMisePackageManagers(t *testing.T) {
	for _, test := range []struct {
		name    string
		tools   string
		command string
		viaMise bool
		runArgs []string
	}{
		{
			name:    "pnpm before node",
			tools:   `{"node":[{"installed":true,"active":true}],"pnpm":[{"installed":true,"active":true}]}`,
			command: "pnpm",
			viaMise: true,
			runArgs: []string{"exec", "--", "pnpm", "install"},
		},
		{
			name:    "yarn",
			tools:   `{"yarn":[{"installed":true,"active":true}]}`,
			command: "yarn",
			viaMise: true,
			runArgs: []string{"exec", "--", "yarn", "install"},
		},
		{
			name:    "bun",
			tools:   `{"bun":[{"installed":true,"active":true}]}`,
			command: "bun",
			viaMise: true,
			runArgs: []string{"exec", "--", "bun", "install"},
		},
		{
			name:    "node means npm",
			tools:   `{"node":[{"installed":true,"active":true}]}`,
			command: "npm",
			viaMise: true,
			runArgs: []string{"exec", "--", "npm", "install"},
		},
		{
			name:    "no tools falls back to direct npm",
			tools:   `{}`,
			command: "npm",
			runArgs: []string{"install"},
		},
		{
			name:    "inactive package manager is ignored",
			tools:   `{"pnpm":[{"installed":true,"active":false}],"node":[{"installed":true,"active":true}]}`,
			command: "npm",
			viaMise: true,
			runArgs: []string{"exec", "--", "npm", "install"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := DetectNodePackageManager(t.TempDir(), outputRunner(test.tools, nil))
			if got := manager.Command(); got != test.command {
				t.Fatalf("command = %q, want %q", got, test.command)
			}
			if got := manager.ViaMise(); got != test.viaMise {
				t.Fatalf("ViaMise = %v, want %v", got, test.viaMise)
			}

			command, args, env := manager.InstallCommand(func(string, []string, commands.Options) error {
				return nil
			})
			if test.viaMise {
				if command != "mise" {
					t.Fatalf("install command = %q, want mise", command)
				}
			} else if command != test.command {
				t.Fatalf("install command = %q, want %q", command, test.command)
			}
			if !reflect.DeepEqual(args, test.runArgs) {
				t.Fatalf("install args = %#v, want %#v", args, test.runArgs)
			}
			if len(env) != 0 {
				t.Fatalf("env = %#v, want none for injected runner", env)
			}
		})
	}
}

func TestDetectNodePackageManagerFallsBackWhenMiseQueryFails(t *testing.T) {
	manager := DetectNodePackageManager(t.TempDir(), outputRunner("", errors.New("mise failed")))
	if manager.Command() != "npm" {
		t.Fatalf("command = %q, want npm", manager.Command())
	}
	if manager.ViaMise() {
		t.Fatal("ViaMise = true, want false")
	}
}

func outputRunner(output string, err error) commands.OutputRunner {
	return func(command string, args []string, options commands.Options) ([]byte, error) {
		return []byte(output), err
	}
}
