# App Anatomy

GoLazy applications use a conventional layout so an agent can find ownership
without inventing another architecture.

## Ownership

| Path | Responsibility |
| --- | --- |
| `services/` | Business rules, use cases, domain models, repositories, data access, and domain-facing integrations. |
| `app/controllers/` | HTTP authorization, request translation, response status, redirects, content negotiation, and view data. |
| `app/views/` | HTML, text, Turbo Stream, mail, and MCP UI presentation. |
| `app/js/` | App-owned browser behavior. |
| `app/styles/` | Tailwind source. |
| `app/public/` | Embedded public files and generated frontend output. |
| `app/mailers/` | Message composition and delivery adapters. |
| `app/jobs/` | Typed scheduling/execution adapters that call services. |
| `app/mcps/` | MCP tools, resources, prompts, and UI adapters that call services. |
| `init/` | Composition: app config, dependency startup, routes, SEO defaults. |
| `cmd/app/` | Thin executable entrypoint. |
| `test/` | Full application HTTP tests. |

The boundary is behavioral, not merely a directory preference. A controller
may choose a status code or view, but it must not decide a discount, account
state transition, voting rule, publication rule, or other domain outcome. A
job may decode its payload and call a service, but the service performs the
work. A mailer may format a message, but a service decides whether and when the
business event occurs.

Authorization checks may live in base-controller generators or controller
hooks. Reusable domain policy still belongs in a service.

## Startup

1. `cmd/app` calls `appinit.App().ListenAndServe()`.
2. `init/app.go` calls `lazyapp.New` with embedded views, public files,
   dependencies, routes, and optional jobs, SEO, PWA, or MCP configuration.
3. `init/dependencies.go` starts services through `lazydeps.Service` and builds
   the typed application context.
4. `init/routes.go` registers resource routes through `Draw`.
5. GoLazy constructs request-local controllers and renders embedded views.

## Feature Recipe

For a normal domain-backed screen:

1. Add the operation and tests in `services/<domain>/`.
2. Add or update the service `New` constructor and typed context helpers.
3. Register the service in `init/dependencies.go`.
4. Add a controller constructor that resolves the service.
5. Add a conventional resource action that calls the service and sets view
   data.
6. Register the resource in `init/routes.go`.
7. Add the matching view and a full app test.

Skip a layer only when the change genuinely does not use it. A static template
still uses a resource controller; it simply may not need a service operation.

## Related

[Services](services.md) | [Routes](routes.md) | [Controllers](controllers.md) |
[Views](views.md) | [Testing](testing-verification.md)
