package main

import lazyconfig "golazy.dev/lazyconfig"

var Config = lazyconfig.MustGetenv[struct {
	Addr             string `default:"127.0.0.1:3000"`
	GoWork           string
	LazyCmd          string
	LazyMultiversion bool `default:"true"`
	LazyTmux         bool
	LazyTmuxSession  string
	Port             int `default:"0"`
}]()
