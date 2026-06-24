package main

import (
	"strings"

	envconfig "golazy.dev/lazyconfig"
)

type envConfig struct {
	Addr             string
	GoWork           string
	LazyCmd          string
	LazyMultiversion string
	LazyTmux         string
	LazyTmuxSession  string
	Port             string
}

func loadConfig() (envConfig, error) {
	return envconfig.Getenv[envConfig]()
}

func (c envConfig) lazyCmdTarget() string {
	return strings.TrimSpace(c.LazyCmd)
}

func (c envConfig) multiversionOff() bool {
	return strings.EqualFold(strings.TrimSpace(c.LazyMultiversion), "off")
}

func (c envConfig) inLazyTmux() bool {
	return strings.TrimSpace(c.LazyTmux) == "1"
}

func (c envConfig) tmuxSession() string {
	return strings.TrimSpace(c.LazyTmuxSession)
}
