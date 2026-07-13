# GoLazy Framework Skill

Use this skill when adding or changing functionality in a GoLazy application.
It is written for coding agents that need to understand where each piece
belongs before editing code.

## First Move

1. Read `AGENTS.md` and `README.md` in the application root.
2. Read only the references in this skill that match the task.
3. Identify whether the change is application behavior or framework-generic
   behavior. Application-specific behavior belongs in this app. Reusable
   controller, routing, rendering, asset, mailer, job, migration, or storage
   behavior belongs in GoLazy packages, not in generated application code.
4. Plan the feature by ownership boundary: service/domain first, dependency
   wiring second, route/controller/view third, tests last.

## References

- `references/framework-parts.md`: what the major GoLazy packages own and when
  to reach for each one.
- `references/app-anatomy.md`: generated app directories, startup flow, and
  where new files usually belong.
- `references/controllers-routing-views.md`: route declarations, controller
  constructors, action methods, generator arguments, template data, layouts,
  errors, and helpers.
- `references/services-context.md`: top-level services, typed context helpers,
  dependency initialization, shared state, and service tests.
- `references/assets-development.md`: embedded public files, Tailwind, lazy js,
  importmaps, `lazy` development behavior, and generated assets.
- `references/cheatsheet-snippets.md`: how generated-app snippets on
  golazy.dev are maintained from sample-app deltas.
- `references/testing-verification.md`: focused package tests, full HTTP tests,
  route/asset checks, and verification commands.

## Feature Recipe

For most application features:

1. Add or update a top-level package under `services/` for business behavior.
2. Register long-lived service instances in `init/dependencies.go` with
   `lazydeps.Service`.
3. Add or update a controller package under `app/controllers/`. Constructors
   receive only `context.Context`, embed `controllers.BaseController`, and
   resolve required services from typed context helpers.
4. Register routes in `init/routes.go` through `Draw`. Use resource routes for
   conventional CRUD and explicit verb routes for custom endpoints.
5. Add views under `app/views/<controller>/<action>.html.tpl` and use
   `Set("name", value)` from the controller to pass template data.
6. Add browser code under `app/js` and Tailwind source under `app/styles` when
   the feature needs frontend behavior. Regenerate public outputs with
   `lazy js` or `lazy tailwind` when their inputs change.
7. Add focused service tests beside the service and app-level HTTP tests under
   `test/` with `lazytest`.

## Guardrails

- Keep generated apps small unless the task asks for a larger product slice.
- Do not add broad framework abstractions to the sample app.
- Do not share controller instances or mutable render state between requests.
- Prefer typed form generators for submitted input. Validate form structs with
  `lazyerrors.Validator(form)` and optional form-owned `Validate() error`
  methods. Validation failures after create or update should set
  `http.StatusUnprocessableEntity` and render the form view with the joined
  validation error; successful writes should redirect with a named route and
  `http.StatusSeeOther`.
- Use controller session and flash helpers (`SessionGet`, `SessionSet`,
  `SessionDelete`, `FlashSet`, `FlashGet`) in controller code, and keep auth
  wrapper types such as `AuthenticatedUser` in application packages.
- Do not use `context.Context` as a general parameter bag; use it for
  initialized services and framework infrastructure.
- Do not edit generated assets by hand when the source manifest, JavaScript,
  package files, or Tailwind input should be changed instead.
- Keep production secrets outside source. Development examples may live in
  `mise.toml` or `.secrets/development.env`.
- Prefer standard-library HTTP, templates, and focused dependencies.
- When changing a public GoLazy API, generated-app convention, or sample-app
  pattern, review the latest guides and the public cheatsheet snippets
  together so copied "how can I?" examples do not drift from this skill.

## Useful Commands

```sh
lazy routes
lazy docs
lazy docs controller
lazy tailwind
lazy js
go test ./...
go test -race ./...
go vet ./...
```
