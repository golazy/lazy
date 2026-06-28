package commands

import "golazy.dev/lazy/services/execservice"

type Options = execservice.Options
type Runner = execservice.Runner
type OutputRunner = execservice.OutputRunner
type ExitError = execservice.ExitError

var Exec = execservice.Exec
var ExecOutput = execservice.ExecOutput
