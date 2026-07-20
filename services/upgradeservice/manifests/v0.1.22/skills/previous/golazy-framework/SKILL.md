---
name: golazy-framework
description: Develop, extend, debug, or review a GoLazy application using its generated-app conventions. Use for app architecture, add-ons, services and dependency lifecycle, routes, controllers, views, forms, SEO, caching, Turbo, Tailwind, JavaScript, assets, mailers, jobs, PWA, MCP, and application tests.
---

# Develop A GoLazy Application

Treat the application as two layers:

- `services/` owns the domain: business rules, use cases, domain models, data
  access, and external systems used to perform business work.
- `app/` owns presentation and communication: controllers, views, forms,
  browser code, mail rendering, job adapters, and MCP adapters.

Authorization may stay at the presentation boundary. Mailers and other
communication adapters may format and deliver messages. Neither should
reimplement business rules. `init/` composes the application; it does not own
business behavior.

## Start Here

1. Read `AGENTS.md` in the application root.
2. Read [App Anatomy](references/app-anatomy.md).
3. Read only the topic files needed for the change. Every topic below links to
   its closest related topics.
4. For behavior changes, begin with [Services](references/services.md), then
   wire dependencies, routes, controllers, and views in that order.
5. Finish with [Views](references/views.md) when rendering changes and
   [Testing](references/testing-verification.md) for every change.

All paths in this skill are relative to the application root. The skill is
self-contained: do not search for a sibling framework checkout, website
repository, or workspace documentation to perform ordinary app work. Use
`lazy docs <package>` when an uncommon API needs exact local documentation.

## Application Workflow

1. Define or extend one focused domain service under `services/<name>/`.
2. Give the service a `New` constructor with the `lazydeps` lifecycle return
   shape when practical, including typed context installation and cleanup.
3. Register it in `init/dependencies.go`. Use dependency references inside
   another service initializer so startup and reverse-order shutdown remain
   explicit.
4. Add a request-local controller under `app/controllers/<name>_controller/`.
   Resolve services in its constructor and keep actions thin.
5. Register the controller with `router.Resources`. Add collection, member, or
   nested routes through the resource callback. Use top-level verb routes only
   when the endpoint is not meaningfully a resource.
6. Add templates under `app/views`, browser behavior under `app/js`, and styles
   under `app/styles`.
7. Test domain behavior beside the service and test the composed HTTP behavior
   under `test/`.

## Topic Index

### Structure

- [App Anatomy](references/app-anatomy.md): ownership boundaries, startup, and
  the end-to-end feature recipe.
- [Framework Parts](references/framework-parts.md): which GoLazy package owns
  each framework concern.
- [Views](references/views.md): templates, layouts, partials, data, and choosing
  another view from a conventional action.
- [Testing](references/testing-verification.md): service and full-app tests.

### Routes And Controllers

- [Routes](references/routes.md)
- [Controllers](references/controllers.md)
- [Controllers/Base](references/controllers-base.md)
- [Controllers/Generators](references/controllers-generators.md)
- [Controllers/BeforeFilters](references/controllers-beforefilters.md)
- [Controllers/ContentTypes](references/controllers-contenttypes.md)
- [Controllers/LazyLoading](references/controllers-lazyloading.md)
- [Controllers/Session](references/controllers-session.md)
- [Controllers/Variants](references/controllers-variants.md)
- [Forms](references/forms.md)

### SEO And Cache

- [SEO](references/seo.md)
- [SEO/href-lang-tags](references/seo-href-lang-tags.md)
- [SEO/alternates](references/seo-alternates.md)
- [SEO/Metadata (JSONLD)](references/seo-metadata-jsonld.md)
- [Cache/Actions](references/cache-actions.md)
- [Cache/Views](references/cache-views.md)

### Turbo

- [Turbo/Visits](references/turbo-visits.md)
- [Turbo/Forms](references/turbo-forms.md)
- [Turbo/Frames](references/turbo-frames.md)
- [Turbo/Frames/Controllers](references/turbo-frames-controllers.md)
- [Turbo/Streams](references/turbo-streams.md)
- [Turbo/Streams/Controller](references/turbo-streams-controller.md)
- [Turbo/ControllerFrames](references/turbo-controllerframes.md)
- [Turbo/Controller](references/turbo-controller.md)

### Frontend And Assets

- [Tailwind](references/tailwind.md)
- [lazyshaft](references/lazyshaft.md)
- [Js](references/js.md)
- [Assets](references/assets.md)

### Runtime Adapters

- [Add-ons](references/addons.md)
- [Mailers](references/mailers.md)
- [Jobs](references/jobs.md)
- [Services](references/services.md)
- [PWA](references/pwa.md)
- [MCP](references/mcp.md)

## Non-Negotiable Conventions

- Put business decisions in `services/`, never in controllers, templates,
  mailers, jobs, MCP tools, `init/`, or `main`.
- Keep controllers concerned with authorization, request/response behavior,
  presentation data, content negotiation, and service orchestration.
- Prefer conventional resource actions: `Index`, `New`, `Create`, `Show`,
  `Edit`, `Update`, and `Delete`.
- Do not add one `Get` route and one custom action for every static-looking
  page. Model pages as a resource, use `Show`, and select the view with
  `Render` or `Variants` in the controller.
- Use typed generators for route values, loaded records, users, and forms.
- Use named route helpers and asset helpers instead of hard-coded generated
  paths.
- Never edit generated CSS, importmaps, or lazyshaft output when a source file
  or manifest owns the change.
- Keep services safe for concurrent requests and controllers request-local.

## Verify

Run the smallest relevant checks, then the whole app suite:

```sh
lazy routes
lazy tailwind  # when Tailwind inputs changed
lazy js        # when JavaScript inputs changed
go test ./...
go test -race ./...
go vet ./...
```
