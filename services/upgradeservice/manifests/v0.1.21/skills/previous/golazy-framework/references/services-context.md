# Services And Context

Application services live in top-level `services/` packages. They own business
operations and should not depend on HTTP controller details.

## Service Package Pattern

```go
package postservice

import "context"

type Service interface {
	List() []Post
}

type service struct{}
type contextKey struct{}

func New() Service {
	return service{}
}

func WithContext(ctx context.Context, service Service) context.Context {
	return context.WithValue(ctx, contextKey{}, service)
}

func FromContext(ctx context.Context) (Service, bool) {
	service, ok := ctx.Value(contextKey{}).(Service)
	return service, ok
}
```

Use an unexported context key type to avoid collisions. Export only the service
API that application code needs.

## Dependency Initialization

Register services once in `init/dependencies.go`:

```go
func Dependencies(deps *lazydeps.Scope) error {
	_, err := lazydeps.Service(deps, "postservice", func(ctx context.Context) (
		context.Context,
		postservice.Service,
		error,
		context.CancelFunc,
	) {
		service := postservice.New()
		return postservice.WithContext(ctx, service), service, nil, nil
	})
	return err
}
```

If one service needs another during startup, call the other service reference's
`Use` method inside the dependent initializer so `lazydeps` records the graph.

Return a cleanup function when the service owns a pool, background process,
open file, network listener, or other resource that must stop on shutdown.

## Shared Versus Request-Local State

- Shared services must be safe for concurrent requests.
- Controllers are request-local and may hold mutable render state.
- Request values and optional action parameters belong in actions, not in the
  app context.
- View data belongs in controller `Set`, not in context.

Run race tests when shared service behavior changes:

```sh
go test -race ./...
```

## Service Tests

Put focused tests beside the service package. Keep them independent from
templates and HTTP routing unless the behavior is truly request-bound.

Use full app tests under `test/` when the feature depends on dependency wiring,
routes, sessions, rendering, assets, or redirects.

## Common Extensions

- Config: use `lazyconfig` for typed environment-backed config structs.
- Auth: `lazyapp.New` always stores a `lazyauth.Config` in app context. Read it
  with `lazyauth.ConfigFromContext(ctx)` when a controller or service should
  authenticate against the app backend. The default backend is in-memory with
  zero users unless `LAZYAUTH_DEFAULT_PASS` seeds the bootstrap user.
- Sessions: enable `lazyapp.Config.Sessions`, then use controller
  `SessionGet`, `SessionSet`, `SessionDelete`, `FlashSet`, and `FlashGet` in
  controller code. Use low-level `lazysession.Get(r)` only for raw handlers
  that intentionally mutate browser session state directly.
- Mail: initialize a `lazymailer.Mailer` and app-specific mailer services from
  dependencies.
- Jobs: define job functions in application packages and wire job config from
  app dependencies.
- Storage and media: initialize `lazystorage`, `lazyfiles`, or `lazymedia`
  services once, then expose app-level operations to controllers.
