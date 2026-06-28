package appinit

import (
	"context"

	"golazy.dev/lazy/app"

	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazyapp"
	"golazy.dev/lazyassets"
	"golazy.dev/lazydeps"

	_ "golazy.dev/lazyview/gotmpl"
)

type Config struct {
	Store             *buildservice.Store
	Actions           buildservice.Actions
	ForceDetailErrors bool
}

func App(config Config) *lazyapp.App {
	if config.Store == nil {
		config.Store = buildservice.NewStore(200)
	}
	if config.Actions == nil {
		config.Actions = buildservice.NewActions()
	}
	return lazyapp.New(lazyapp.Config{
		Name:              "golazy.dev/lazy/dev-panel",
		Drawer:            Draw,
		Public:            app.Public,
		Views:             app.Views,
		Dependencies:      dependencies(config),
		ForceDetailErrors: config.ForceDetailErrors,
		AssetOptions: []lazyassets.Option{
			lazyassets.WithURLPrefix("/_golazy"),
			lazyassets.WithDevelopmentMode(true),
		},
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
