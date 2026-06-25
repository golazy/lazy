package main

import (
	"os"
	"testing"

	"golazy.dev/lazy/commands/lazytmux"
	lazyconfig "golazy.dev/lazyconfig"
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
	reloadConfigForTest(t)

	if Config.Addr != "127.0.0.1:4000" {
		t.Fatalf("Addr = %q", Config.Addr)
	}
	if Config.GoWork != "/tmp/example/go.work" {
		t.Fatalf("GoWork = %q", Config.GoWork)
	}
	if Config.LazyCmd != "/tmp/lazy" {
		t.Fatalf("LazyCmd = %q", Config.LazyCmd)
	}
	if Config.LazyMultiversion {
		t.Fatalf("LazyMultiversion = true")
	}
	if !Config.LazyTmux {
		t.Fatalf("LazyTmux = false")
	}
	if Config.LazyTmuxSession != "lazy-app" {
		t.Fatalf("LazyTmuxSession = %q", Config.LazyTmuxSession)
	}
	if Config.Port != 5000 {
		t.Fatalf("Port = %d", Config.Port)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	unsetenv(t, addrEnv, lazyMultiversionEnv, portEnv)
	reloadConfigForTest(t)

	if Config.Addr != "127.0.0.1:3000" {
		t.Fatalf("Addr = %q", Config.Addr)
	}
	if !Config.LazyMultiversion {
		t.Fatalf("LazyMultiversion = false")
	}
	if Config.Port != 0 {
		t.Fatalf("Port = %d", Config.Port)
	}
}

func reloadConfigForTest(t *testing.T) {
	t.Helper()
	oldConfig := Config
	Config = lazyconfig.MustGetenv[struct {
		Addr             string `default:"127.0.0.1:3000"`
		GoWork           string
		LazyCmd          string
		LazyMultiversion bool `default:"true"`
		LazyTmux         bool
		LazyTmuxSession  string
		Port             int `default:"0"`
	}]()
	t.Cleanup(func() {
		Config = oldConfig
	})
}

func unsetenv(t *testing.T, names ...string) {
	t.Helper()
	oldValues := make(map[string]string, len(names))
	hadValues := make(map[string]bool, len(names))
	for _, name := range names {
		oldValues[name], hadValues[name] = os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, name := range names {
			if hadValues[name] {
				_ = os.Setenv(name, oldValues[name])
			} else {
				_ = os.Unsetenv(name)
			}
		}
	})
}
