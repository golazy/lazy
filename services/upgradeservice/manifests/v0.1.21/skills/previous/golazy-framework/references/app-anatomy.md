# App Anatomy

Generated GoLazy apps have ordinary Go modules with a conventional layout.
The convention is useful because `lazyapp`, `lazyroutes`, `lazycontroller`,
`lazyview`, and the CLI can infer the common pieces.

## Directory Ownership

- `cmd/app` or `cmd/<app-name>`: executable entrypoint. Keep it thin.
- `init/app.go`: builds the `*lazyapp.App` with `lazyapp.New`.
- `init/dependencies.go`: initializes shared app services with `lazydeps`.
- `init/routes.go`: declares the route table in `Draw`.
- `app/controllers`: web controllers and shared base controller behavior.
- `app/views`: controller views and layouts.
- `app/public`: embedded public files and generated browser assets.
- `app/js`: app-owned JavaScript source consumed by `lazy js`.
- `app/styles`: stylesheet source consumed by `lazy tailwind`.
- `services`: business behavior outside the web presentation layer.
- `test`: full application HTTP tests.
- `.mise/tasks`: project task scripts and service lifecycle commands.
- `datasets`: optional local development snapshots, one file per service.

## Startup Flow

1. `cmd/app` calls `appinit.App()` and starts the returned app.
2. `init/app.go` passes app name, route drawer, embedded views, embedded public
   files, and dependencies to `lazyapp.New`.
3. `lazyapp.New` initializes the dependency scope and app context.
4. `lazyapp.New` installs the app `lazyauth` config. Without `Config.Auth`, the
   default in-memory backend starts with zero users; `LAZYAUTH_DEFAULT_PASS`
   creates a bootstrap `admin` user, and `LAZYAUTH_DEFAULT_USER` changes that
   username.
5. `lazyapp.New` registers helpers, opens the view renderer, registers public
   assets, builds the route scope, and calls `Draw`.
6. Requests first try dynamic application routes. Public files are served as
   the final application fallback.

## Where To Put Features

- Business rule, external API wrapper, repository, or model logic:
  `services/<name>`.
- Service construction, app-level config, database pools, mail registries,
  storage registries, job config: `init/dependencies.go` or a focused helper
  called from there.
- HTTP request handling and template data: `app/controllers/<name>`.
- URLs and route names: `init/routes.go`.
- HTML output: `app/views`.
- Browser interactions: `app/js` and generated output under `app/public`.
- Styling: `app/styles` and generated `app/public/styles.css`.
- End-to-end behavior tests: `test`.

## Common Application Slice

For a new page backed by domain logic:

1. Create a service with a small public API and focused tests.
2. Add typed `WithContext` and `FromContext` helpers in that service package.
3. Register the service in `Dependencies`.
4. Create a controller that embeds `controllers.BaseController`.
5. Resolve the service in `New(ctx context.Context)`.
6. Add an action method that sets view data and returns `nil`.
7. Add a route in `Draw`.
8. Add the matching template and an HTTP test.

Keep this order unless the task is clearly only a route, only a view, or only a
service.
