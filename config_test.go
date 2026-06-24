package main

import (
	"testing"

	"golazy.dev/lazy/commands/lazytmux"
)

const (
	addrEnv              = "ADDR"
	goWorkEnv            = "GOWORK"
	lazyCmdEnv           = "LAZYCMD"
	lazyMultiversionEnv  = "LAZY_MULTIVERSION"
	lazyMultiversionOff  = "off"
	lazyTmuxSessionEnv   = lazytmux.SessionEnv
	lazyTmuxInSessionEnv = lazytmux.InSessionEnv
	portEnv              = "PORT"
)

func TestLoadConfigFromEnvironment(t *testing.T) {
	t.Setenv(addrEnv, "127.0.0.1:4000")
	t.Setenv(goWorkEnv, "/tmp/example/go.work")
	t.Setenv(lazyCmdEnv, "/tmp/lazy")
	t.Setenv(lazyMultiversionEnv, "off")
	t.Setenv(lazyTmuxInSessionEnv, "1")
	t.Setenv(lazyTmuxSessionEnv, "lazy-app")
	t.Setenv(portEnv, "5000")

	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if config.Addr != "127.0.0.1:4000" {
		t.Fatalf("Addr = %q", config.Addr)
	}
	if config.GoWork != "/tmp/example/go.work" {
		t.Fatalf("GoWork = %q", config.GoWork)
	}
	if config.lazyCmdTarget() != "/tmp/lazy" {
		t.Fatalf("lazyCmdTarget() = %q", config.lazyCmdTarget())
	}
	if !config.multiversionOff() {
		t.Fatalf("multiversionOff() = false")
	}
	if !config.inLazyTmux() {
		t.Fatalf("inLazyTmux() = false")
	}
	if config.tmuxSession() != "lazy-app" {
		t.Fatalf("tmuxSession() = %q", config.tmuxSession())
	}
	if config.Port != "5000" {
		t.Fatalf("Port = %q", config.Port)
	}
}
