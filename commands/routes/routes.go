package routes

import (
	"io"

	"golazy.dev/lazy/services/routesservice"
)

type Command = routesservice.Command
type Route = routesservice.Route

func parseRoutes(output []byte) ([]Route, error) {
	return routesservice.ParseRoutes(output)
}

func writeTable(w io.Writer, routes []Route) error {
	return routesservice.WriteTable(w, routes)
}
