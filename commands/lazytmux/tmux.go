package lazytmux

import (
	"golazy.dev/lazy/commands/lazyconfig"
	"golazy.dev/lazy/services/tmuxservice"
)

const InSessionEnv = tmuxservice.InSessionEnv
const SessionEnv = tmuxservice.SessionEnv

type Command = tmuxservice.Command

func servicePreparedAppCommand(services []lazyconfig.Service, session string, cmdPath string, viewPath string, publicPath string) string {
	return tmuxservice.ServicePreparedAppCommand(services, session, cmdPath, viewPath, publicPath)
}
