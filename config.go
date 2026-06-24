package main

import (
	"strings"

	"golazy.dev/lazy/commands/lazytmux"
	envconfig "golazy.dev/lazyconfig"
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

type envConfig struct {
	Addr             string `var:"ADDR"`
	GoWork           string `var:"GOWORK"`
	LazyCmd          string `var:"LAZYCMD"`
	LazyMultiversion string `var:"LAZY_MULTIVERSION"`
	LazyTmux         string `var:"LAZY_TMUX"`
	LazyTmuxSession  string `var:"LAZY_TMUX_SESSION"`
	Port             string `var:"PORT"`
}

func loadConfig() (envConfig, error) {
	return envconfig.Getenv[envConfig]()
}

func (c envConfig) lazyCmdTarget() string {
	return strings.TrimSpace(c.LazyCmd)
}

func (c envConfig) multiversionOff() bool {
	return strings.EqualFold(strings.TrimSpace(c.LazyMultiversion), lazyMultiversionOff)
}

func (c envConfig) inLazyTmux() bool {
	return strings.TrimSpace(c.LazyTmux) == "1"
}

func (c envConfig) tmuxSession() string {
	return strings.TrimSpace(c.LazyTmuxSession)
}
