package lazyconfig

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	config, err := Parse([]byte(`
services = ["postgres", "s3"]

[tmux]
session = "shop"

[[runners]]
name = "tailwind"
command = "lazy tailwind --watch"

[[programs]]
name = "editor"
command = "nvim"
window = "workspace"
`))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := config.Tmux.Session, "shop"; got != want {
		t.Fatalf("session = %q, want %q", got, want)
	}
	if got, want := ServiceNames(config), []string{"postgres", "s3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %#v, want %#v", got, want)
	}
	if got, want := config.Runners, []Process{{Name: "tailwind", Command: "lazy tailwind --watch"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("runners = %#v, want %#v", got, want)
	}
	if got, want := config.Programs, []Program{{Name: "editor", Command: "nvim", Window: "workspace"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("programs = %#v, want %#v", got, want)
	}
}

func TestParseServiceArrayTables(t *testing.T) {
	config, err := Parse([]byte(`
[[services]]
name = "postgres"

[[services]]
name = "redis"
`))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ServiceNames(config), []string{"postgres", "redis"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %#v, want %#v", got, want)
	}
}

func TestParseRejectsDuplicateServices(t *testing.T) {
	_, err := Parse([]byte(`services = ["postgres", "postgres"]`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `service "postgres" is already declared`) {
		t.Fatalf("error = %v", err)
	}
}
