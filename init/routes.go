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
	"golazy.dev/lazysupport/inflection"
)

func init() {
	inflection.Irregular("console", "console")
	inflection.Irregular("status", "status")
}

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
			resource.Get("request-monitoring", (*panelcontroller.Controller).RequestMonitoring)
			resource.Post("request-monitoring/on", (*panelcontroller.Controller).RequestMonitoringOn)
			resource.Post("request-monitoring/off", (*panelcontroller.Controller).RequestMonitoringOff)
			resource.Post("rebuild", (*panelcontroller.Controller).Rebuild)
			resource.Post("restart", (*panelcontroller.Controller).Restart)
		})
		panel.Resources(requests.New)
		panel.Resources(console.New)
		panel.Resources(logs.New)
		panel.Resources(services.New)
		panel.Resources(traces.New)
		panel.Resources(routes.New)
		panel.Resources(jobs.New)
		panel.Resources(assets.New)
		panel.Resources(actions.New)
		panel.Resources(status.New)
	})
}
