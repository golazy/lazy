package commands

import "golazy.dev/lazy/services/execservice"

func ResolveMiseCommand() (string, []string) {
	return execservice.ResolveMiseCommand()
}

func MiseExecCommand(command string, args []string) (string, []string, []string) {
	return execservice.MiseExecCommand(command, args)
}

func MiseExecRunnerCommand(runner Runner, command string, args []string) (string, []string, []string) {
	return execservice.MiseExecRunnerCommand(runner, command, args)
}

func MiseExecOutputRunnerCommand(runner OutputRunner, command string, args []string) (string, []string, []string) {
	return execservice.MiseExecOutputRunnerCommand(runner, command, args)
}
