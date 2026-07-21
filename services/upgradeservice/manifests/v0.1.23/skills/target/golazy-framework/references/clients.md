# Clients

Use `clients/` for focused, low-level integrations with external systems. A
client may own provider authentication, SDK setup, HTTP behavior, wire types,
parsing, pagination, protocol errors, tracing hooks, and shutdown details.
Business decisions and use-case orchestration stay in `services/`.

Do not wrap every dependency. Add an application-owned boundary only when it
owns useful behavior or semantics. Prefer direct Go types over wrappers that
only rename, embed, or forward an existing client.

## Dependency Direction

- `init/` reads configuration, constructs clients, registers cleanup, and
  injects dependencies.
- `services/` use clients to perform business work.
- `app/` normally resolves services rather than clients.
- Direct client use from `app/` is an explicit exception for presentation- or
  transport-specific behavior such as an OAuth redirect flow.

Clients are ordinary structs. Provide `New` or `Open` when construction must
validate configuration, install hooks, prepare a transport, or establish a
resource. Allow direct struct construction or a useful zero value when no
constructor invariant exists. Read environment variables in `init/` and pass
explicit configuration into the client.

## Choose The Exposed Type

Prefer a service-owned request or result type when:

- The value has business meaning or enforces domain invariants.
- The service combines multiple sources or translates provider failures into
  domain behavior.
- Provider fields should not spread into controllers, jobs, or other services.
- Replacing the provider is plausible and a stable application contract helps.

Reuse a type from `clients/<provider>` when:

- It represents normalized integration data rather than a business entity.
- The client has already removed irrelevant SDK or wire details.
- A service-owned type would copy the same fields without adding semantics.
- Multiple services legitimately use the same integration value.

Expose an upstream type or the upstream client directly when:

- It is already the canonical Go abstraction, such as `*pgxpool.Pool`,
  `*http.Client`, `io.Reader`, or `fs.FS`.
- The application needs a large, meaningful part of the upstream API.
- A wrapper would mostly contain one-line shadow methods or embed the upstream
  client to recover its API.
- Streaming, transactions, batching, buffers, or another measured performance
  concern would be harmed by translation.
- The dependency and its coupling are stable and intentionally accepted.

Memory or copying cost is a measured exception, not the default reason to leak
an SDK. Pointers, slices, readers, and iterators often preserve efficiency
without exposing the entire provider API.

## Interfaces

Do not create a `Client` interface by default. A service may accept a concrete
client directly. Define the smallest interface at the consuming package when
multiple implementations, provider replacement, or focused service tests make
substitution valuable. Implement only the methods the application uses; do not
mirror a complete vendor SDK in an application interface.

## Lifecycle

Construct long-lived clients in `init/dependencies.go`. Register a client as a
`lazydeps.Service` dependency when it owns a pool, goroutine, transport, open
file, listener, or other cleanup. Pass its `Ref.Use()` value inside the
dependent service initializer so startup and reverse-order shutdown remain
explicit. The client does not need a context helper unless runtime application
code genuinely resolves it from context.

A package such as `clients/postgres` may legitimately provide setup and return
the standard `*pgxpool.Pool`. It can install tracing hooks, configure limits,
run startup checks, and provide `pool.Close` without wrapping `Query`, `Exec`,
`Begin`, or every other pool method.

## Warning Signs

Reconsider the boundary when it creates:

- A struct that duplicates every field of a client or SDK type.
- Methods that only call the identically named upstream methods.
- An interface matching a concrete client's complete method set.
- An embedded upstream client used only to expose methods omitted by a wrapper.
- Provider-specific values in ordinary controller view data by accident.
- Conversion of large results justified only as architecture, without a
  semantic or measured performance benefit.

## Related

[App Anatomy](app-anatomy.md) | [Services](services.md) |
[Testing](testing-verification.md)
