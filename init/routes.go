package appinit

import (
	panelcontroller "golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/app/controllers/panel/actions"
	"golazy.dev/lazy/app/controllers/panel/assets"
	"golazy.dev/lazy/app/controllers/panel/console"
	"golazy.dev/lazy/app/controllers/panel/jobs"
	"golazy.dev/lazy/app/controllers/panel/logs"
	"golazy.dev/lazy/app/controllers/panel/requests"
	"golazy.dev/lazy/app/controllers/panel/routes"
	"golazy.dev/lazy/app/controllers/panel/services"
	"golazy.dev/lazy/app/controllers/panel/status"
	"golazy.dev/lazy/app/controllers/panel/traces"
	"golazy.dev/lazyroutes"
)

func Draw(router *lazyroutes.Scope) {
	router.Path("_golazy", func(panel *lazyroutes.Scope) {
		panel.Resources(panelcontroller.New, func(resource *lazyroutes.Resource) {
			resource.Singular("panel")
			resource.Plural("panel")
			resource.Path("")
			resource.Get("state", (*panelcontroller.Controller).State)
			resource.Get("events", (*panelcontroller.Controller).Events)
			resource.Get("cache", (*panelcontroller.Controller).Cache)
			resource.Post("cache/on", (*panelcontroller.Controller).CacheOn)
			resource.Post("cache/off", (*panelcontroller.Controller).CacheOff)
			resource.Post("rebuild", (*panelcontroller.Controller).Rebuild)
			resource.Post("restart", (*panelcontroller.Controller).Restart)
		})
		panel.Resources(requests.New, namedResource("request", "requests"))
		panel.Resources(console.New, namedResource("console", "console"))
		panel.Resources(logs.New, namedResource("log", "logs"))
		panel.Resources(services.New, namedResource("service", "services"))
		panel.Resources(traces.New, namedResource("trace", "traces"))
		panel.Resources(routes.New, namedResource("route", "routes"))
		panel.Resources(jobs.New, namedResource("job", "jobs"))
		panel.Resources(assets.New, namedResource("asset", "assets"))
		panel.Resources(actions.New, namedResource("action", "actions"))
		panel.Resources(status.New, namedResource("status", "status"))
	})
}

func namedResource(singular string, plural string) func(*lazyroutes.Resource) {
	return func(resource *lazyroutes.Resource) {
		resource.Singular(singular)
		resource.Plural(plural)
	}
}
