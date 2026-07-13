package addons_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	addonscommand "golazy.dev/lazy/commands/addons"
	"golazy.dev/lazy/services/addonservice"
)

func TestAddDryRunAndApplyThenList(t *testing.T) {
	dir := commandFixture(t)
	command := addonscommand.Command{
		Dir:     dir,
		Sources: []addonservice.Source{addonservice.FirstPartySource("v0.1.21")},
	}

	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	code, err := command.Execute("add", []string{"--dry-run", "postgres"})
	if err != nil || code != 0 {
		t.Fatalf("dry-run = %d, %v; stderr=%s", code, err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "would create addons.toml") || !strings.Contains(stdout.String(), "would update go.mod") {
		t.Fatalf("dry-run output =\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "addons.toml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote addons.toml: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code, err = command.Execute("add", []string{"--config", "database_url_variable=APP_DATABASE_URL", "postgres"})
	if err != nil || code != 0 {
		t.Fatalf("add = %d, %v; stderr=%s", code, err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created addons.toml") || !strings.Contains(stdout.String(), "created .mise/tasks/postgres/start") {
		t.Fatalf("add output =\n%s", stdout.String())
	}
	state, err := addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := state.Addons[0].Config["database_url_variable"]; got != "APP_DATABASE_URL" {
		t.Fatalf("locked add-on configuration = %q", got)
	}
	generated, err := os.ReadFile(filepath.Join(dir, addonservice.GeneratedFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `"database_url_variable": "APP_DATABASE_URL"`) {
		t.Fatalf("generated configuration =\n%s", generated)
	}

	stdout.Reset()
	code, err = command.Execute("add", []string{"--unset-config", "database_url_variable", "postgres"})
	if err != nil || code != 0 {
		t.Fatalf("unset config = %d, %v", code, err)
	}
	state, err = addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Addons[0].Config) != 0 {
		t.Fatalf("configuration after unset = %#v", state.Addons[0].Config)
	}

	stdout.Reset()
	code, err = command.Execute("addons", nil)
	if err != nil || code != 0 {
		t.Fatalf("addons = %d, %v", code, err)
	}
	if !strings.Contains(stdout.String(), "postgres\tv0.1.21\tdirect\tgolazy.dev/pg@v0.1.21") {
		t.Fatalf("list output =\n%s", stdout.String())
	}
}

func TestCommandUsageErrors(t *testing.T) {
	var stderr bytes.Buffer
	command := addonscommand.Command{Dir: t.TempDir(), Stderr: &stderr}
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "add"},
		{name: "remove"},
		{name: "addons", args: []string{"extra"}},
	} {
		stderr.Reset()
		code, err := command.Execute(test.name, test.args)
		if err != nil || code != 1 {
			t.Fatalf("%s = %d, %v", test.name, code, err)
		}
		if !strings.Contains(stderr.String(), "usage: lazy "+test.name) {
			t.Fatalf("%s stderr = %q", test.name, stderr.String())
		}
	}
}

func commandFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeCommandFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.26.2\n\nrequire golazy.dev v0.1.20\n")
	writeCommandFile(t, dir, "init/app.go", `package appinit

import "golazy.dev/lazyapp"

func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{Name: "app"})
}
`)
	return dir
}

func writeCommandFile(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
