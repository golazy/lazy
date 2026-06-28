package lazyconfig

import "golazy.dev/lazy/services/configservice"

const Filename = configservice.Filename

type Config = configservice.Config
type Tmux = configservice.Tmux
type Service = configservice.Service
type Process = configservice.Process
type Program = configservice.Program

var Load = configservice.Load
var LoadIfExists = configservice.LoadIfExists
var Parse = configservice.Parse
var ServiceNames = configservice.ServiceNames
