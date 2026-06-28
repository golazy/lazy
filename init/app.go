package appinit

import (
	"context"

	"golazy.dev/lazy/app"
	panelcontroller "golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazyapp"
	"golazy.dev/lazyassets"
	"golazy.dev/lazydeps"
	"golazy.dev/lazyroutes"
	_ "golazy.dev/lazyview/gotmpl"
)

type Config struct {
	Store   *buildservice.Store
	Actions buildservice.Actions
}

func App(config Config) *lazyapp.App {
	if config.Store == nil {
		config.Store = buildservice.NewStore(200)
	}
	if config.Actions == nil {
		config.Actions = buildservice.NewActions()
	}
	return lazyapp.New(lazyapp.Config{
		Name:         "golazy.dev/lazy/dev-panel",
		Drawer:       Draw,
		Public:       app.Public,
		Views:        app.Views,
		Dependencies: dependencies(config),
		AssetOptions: []lazyassets.Option{
			lazyassets.WithURLPrefix("/_golazy"),
			lazyassets.WithDevelopmentMode(true),
		},
	})
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
			resource.Get("jobs", (*panelcontroller.Controller).Jobs)
			resource.Post("cache/on", (*panelcontroller.Controller).CacheOn)
			resource.Post("cache/off", (*panelcontroller.Controller).CacheOff)
			resource.Post("rebuild", (*panelcontroller.Controller).Rebuild)
			resource.Post("restart", (*panelcontroller.Controller).Restart)
		})
	})
}

func dependencies(config Config) func(*lazydeps.Scope) error {
	return func(scope *lazydeps.Scope) error {
		_, err := lazydeps.Service(scope, "dev-panel", func(ctx context.Context) (context.Context, struct{}, error, context.CancelFunc) {
			ctx = buildservice.WithStore(ctx, config.Store)
			ctx = buildservice.WithActions(ctx, config.Actions)
			return ctx, struct{}{}, nil, nil
		})
		return err
	}
}
