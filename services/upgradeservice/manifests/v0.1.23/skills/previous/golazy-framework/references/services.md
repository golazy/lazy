# Services

Every business operation belongs in a focused package under `services/`.
Services represent the application's domain and remain usable without HTTP,
templates, mail rendering, jobs, or MCP.

## Contents

- [What Services Own](#what-services-own)
- [Lifecycle-Compatible Constructor](#lifecycle-compatible-constructor)
- [Chain Dependencies](#chain-dependencies)
- [Design And Tests](#design-and-tests)

## What Services Own

- Business rules and state transitions.
- Use cases that coordinate repositories or domain-facing clients.
- Domain models and typed domain errors.
- Data access and persistence interfaces or implementations.
- Concurrency and lifecycle of long-lived domain resources.

Controllers own authorization and presentation. Mailers own communication
formatting. Jobs and MCP modules are adapters. All of them call services for
business work.

## Lifecycle-Compatible Constructor

Prefer a `New` function with the same shape as `lazydeps.Func[T]`:

```go
// services/postservice/postservice.go
package postservice

import "context"

type Service interface {
	Latest(context.Context) ([]Post, error)
	Find(context.Context, int) (*Post, error)
}

type service struct{}
type contextKey struct{}

func New(ctx context.Context) (context.Context, Service, error, context.CancelFunc) {
	service := &service{}
	return WithContext(ctx, service), service, nil, nil
}

func WithContext(ctx context.Context, service Service) context.Context {
	return context.WithValue(ctx, contextKey{}, service)
}

func FromContext(ctx context.Context) (Service, bool) {
	service, ok := ctx.Value(contextKey{}).(Service)
	return service, ok
}
```

This lets the composition root register the constructor directly:

```go
func Dependencies(deps *lazydeps.Scope) error {
	_, err := lazydeps.Service(deps, "postservice", postservice.New)
	return err
}
```

`lazydeps.Service` starts the service by invoking `New`. It cancels the
service context during shutdown, then calls the returned cleanup function.
Return a cleanup function when the service owns a pool, goroutine, open file,
listener, or client that must be closed or awaited.

## Chain Dependencies

When a service needs another initialized dependency, call `Ref.Use()` inside
the dependent initializer. This records the graph and makes shutdown happen in
reverse dependency order.

```go
database, err := lazydeps.Service(deps, "database", databaseservice.New)
if err != nil {
	return err
}

_, err = lazydeps.Service(deps, "postservice", func(ctx context.Context) (
	context.Context,
	postservice.Service,
	error,
	context.CancelFunc,
) {
	return postservice.New(ctx, database.Use())
})
return err
```

For this form, define `postservice.New` with the same lifecycle returns plus
the explicit dependency:

```go
func New(ctx context.Context, database Database) (
	context.Context,
	Service,
	error,
	context.CancelFunc,
) {
	service := &service{database: database}
	return WithContext(ctx, service), service, nil, nil
}
```

Do not call `Use()` outside another `lazydeps.Service` initializer; the scope
uses that call site to record the dependency edge.

## Design And Tests

Keep the public API small and domain-oriented. Do not expose HTTP status codes,
template names, controller types, or form structs from a service. Use `context`
for cancellation and deadlines on I/O operations, not as an untyped parameter
bag.

Services are shared across requests and must be concurrency-safe. Test them
beside the package, including domain failures and lifecycle cleanup. Run
`go test -race ./...` when shared state changes.

## Related

[App Anatomy](app-anatomy.md) | [Controllers](controllers.md) |
[Forms](forms.md) | [Jobs](jobs.md) | [Mailers](mailers.md) | [MCP](mcp.md) |
[Testing](testing-verification.md)
