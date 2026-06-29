# lazy

GoLazy command-line application.

## Run an application

From a Go module:

```sh
lazy
```

The command reads the module path from `go.mod` and first looks for
`./cmd/<module-name>`. If that directory does not exist, it falls back to
`./cmd/app`.

`lazy` is a development runner. It runs `go mod tidy` when Go workspace mode is
not active, builds the app command into a temporary binary, serves the public
`ADDR` or `PORT` through a local proxy, runs the app on an internal loopback
port, watches application files, rebuilds and restarts on changes, and injects
a small reload client into HTML responses. The public proxy accepts HTTP and
HTTPS on the same port. Plain HTTP serves a GoLazy local HTTPS setup page with
the custom certificate authority download; after the browser trusts that CA,
the page redirects to the HTTPS version and the development panel can use
HTTP/2 multiplexing for its streams and app requests. Proxy startup,
generated-asset work, builds, and app starts use compact progress lines.
Successful task output stays hidden, failed task output is printed, and the
application process still writes directly to the terminal. If the app is not
running, the HTTPS proxy serves a status page with the latest build state.
When the development panel is embedded in proxied app pages, drag its top edge
to resize it; `lazy` adjusts the page's bottom padding with the panel height so
app content remains visible.

The local certificate authority is created on demand under the user's data
directory in a `lazy` directory, with strict file permissions. The setup page
prints the exact paths. Never share that directory or the private key.

When an application declares or exposes local services, `lazy` starts those
services as managed subprocesses after the development proxy is already
serving its status page. Service commands are ordinary non-interactive
processes; their stdout and stderr stay attached to the terminal and are
recorded by the development panel. The panel's Services tab shows per-service
stdout and stderr, splits logs by lifecycle script such as `start`, `check`,
`create`, and `migrate`, and includes the run number for each task attempt. The
status bar shows each service with a stopped, not-ready, or ready indicator
that opens the Services tab when clicked. The Services tab can restart one
managed service at a time from the service list.

Example `lazy.toml`:

```toml
services = ["postgres", "s3"]
```

Each service is implemented as mise tasks named `{service}:{action}`. For a
stateful service such as PostgreSQL, define `postgres:start`,
`postgres:check`, `postgres:create`, `postgres:dump FILE`,
`postgres:load FILE`, and `postgres:migrate`. `postgres:start` is mandatory
and must run in the foreground so SIGINT stops it. `postgres:check` should
return status 0 only when the service is active and ready for dependent
processes; the `lazy` command polls it every 500ms before running create,
migrate, or the app when the task exists. If the check is still failing after
five seconds, `lazy` reports that the user should inspect the service output
and keeps checking. Add `postgres:kill` only as an escape hatch for stale local
processes.

When `lazy.toml` lists services, `lazy` uses that list. Otherwise, it
discovers service names from `.mise/tasks` entries ending in `:start`. Services
start in parallel. After a service check succeeds, `lazy` runs that service's
`create` and `migrate` tasks when they exist. Create and migrate failures are
reported, and `lazy` continues preparing the remaining services. The Go app
starts after all discovered services have finished their startup preparation.
On Ctrl-C, `lazy` stops the app first and then stops the services; a second
Ctrl-C escalates to killing child processes.

Use direct Go commands when you want to run without the development proxy or
watcher. For an app generated as `github.com/guillermo/my_app`:

```sh
go run ./cmd/my_app
```

## Inspect routes

From a GoLazy application module:

```sh
lazy routes
```

The command runs the application with the `lazydev,printroutes` build tags,
passes local view and public roots as build-time values, lets `lazyapp.New`
initialize the app and call `Draw`, then prints the route table without
starting the HTTP server.

## Build JavaScript

From a GoLazy application module with `js.toml`:

```sh
lazy js
```

The command installs the JavaScript packages named by `[entrypoint.<name>]`
blocks, bundles those library entrypoints with esbuild, expands the
`// golazy:turbo` and `// golazy:stimulus` directives in `app/js/app.js`, writes
app-owned modules from `app/js` as readable hashed assets, and writes the
importmap.
During `lazy` development, apps with `js.toml` run the JavaScript pipeline
before the first build and after changes to `app/js`, `js.toml`, or package
metadata.

## Build Tailwind styles

From a GoLazy application module:

```sh
lazy tailwind
```

The command installs `tailwindcss` and `@tailwindcss/cli` when they are
missing, creates a Tailwind input stylesheet if needed, and compiles it into
the app's public stylesheet. Conventional apps default to:

```text
app/styles/application.css -> app/public/styles.css
```

Use watch mode during UI work:

```sh
lazy tailwind --watch
```

Override paths when needed:

```sh
lazy tailwind --input app/styles/site.css --output app/public/site.css
```

## Manage local datasets

From a GoLazy application module with service dump/load tasks:

```sh
lazy dump baseline
lazy load baseline
```

`lazy dump <name>` creates `datasets/<name>` and runs each discovered
service's `dump` task with a service-specific output path such as
`datasets/baseline/postgres.dump`. `lazy load <name>` runs matching `load`
tasks for dump files present in the dataset.

## Create an application

```sh
lazy new github.com/guillermo/my_app
lazy new --version v0.1.10 github.com/guillermo/old_app
```

The command checks `https://golazy.dev/lazy.version` with a one-second timeout
before remote template generation. If a newer `lazy` is available, it stops so
new apps start from the current template. Network failures and timeouts are
ignored. Use `--skip-update-check` to bypass the online check.

By default, `lazy new` creates `./my_app` from the `golazy/sample_app` tag
matching the CLI version. Use `--version <version>` to clone a specific sample
app tag instead. The command removes the template Git history, changes the
module and imports, renames the command directory to `cmd/my_app`, trusts the
generated `mise.toml`, runs `mise install`, then validates with the current
`go` on `PATH` by running `go mod tidy` and `go test ./...`. After validation
it initializes a fresh Git repository, commits the generated checkout with a
command-local GoLazy identity, and prints the generated app directory and the
`lazy` command to run next.

If `mise` was just installed by the public installer and the current shell has
not picked up the new `PATH`, `lazy new` can still run setup through
`$HOME/.local/bin/mise`. Open a new shell before running app-level commands
normally, or copy any printed `export PATH=...` lines before running `lazy` in
the current shell.

For local validation against the checked-out sample application, point `lazy
new` at a directory:

```sh
lazy new --source-dir ../sample_app github.com/guillermo/my_app
```

## Upgrade an application

From a GoLazy application module:

```sh
lazy upgrade
lazy upgrade --target v0.1.12
lazy upgrade --force v0.1.10
```

The command reads the current `golazy.dev` requirement from `go.mod`.
Without `--target`, it applies the next supported one-step migration. With
`--target`, it runs each supported step in order until the target version.
Apps that predate `lazy upgrade` start at the first automated migration the
current binary knows about, currently `v0.1.10 -> v0.1.11`, instead of trying
to run older `lazy` binaries that did not have an upgrade command.
Use `--force <version>` to run the one-step migration that starts at that
version even when `go.mod` currently records another version; for example,
`--force v0.1.10` runs the `v0.1.10 -> v0.1.11` migration.

This first implementation carries backfilled migrations for:

- `v0.1.10 -> v0.1.11`: moves sample-app mise tasks from inline
  `mise.toml` entries to `.mise/tasks/dev` and `.mise/tasks/test`, adds
  Node.js to the tool list, and removes Go from app-level mise tools.
- `v0.1.11 -> v0.1.12`: moves generated-app services from `app/services` to
  top-level `services` and rewrites matching Go imports.
- `v0.1.14 -> v0.1.15`: renames `init/context.go` to
  `init/dependencies.go`, rewrites `lazyapp.Config.Context` to
  `lazyapp.Config.Dependencies`, and updates simple initializer returns to the
  new `func(*lazydeps.Scope) error` shape. It also moves inline
  `lazyapp.Config.SEO` option slices into `init/seo.go` and wires `SEO: SEO`.

Template-owned files are manifest-gated with SHA-256 hashes. New files are
created directly. Replacements are applied directly only when the current file
still matches the previous rendered sample-app version. Files removed from the
new sample app are deleted directly only when the current file still matches
the previous rendered sample-app version.

If a replacement file looks customized, `lazy upgrade` prints a diff and asks
whether to install the new version while backing up the current file next to it
as `<filename>-YYYY-MM-DD`, or abort so you can merge manually. If a removed
file looks customized, the command asks whether to delete it with a dated
backup, keep it and continue, or abort. Keeping a removed file can create
issues when the app still loads it. Non-interactive conflicts stop and write
the proposed replacement under `.golazy/upgrade/conflicts`.

After each successful step, `lazy upgrade` runs:

```sh
go mod tidy
go test ./...
go vet ./...
```

Use `--dry-run` to inspect planned writes and `--skip-commands` when you need
to run verification manually. `--force` does not silently overwrite customized
template-owned files; conflicts still require an explicit choice.

## Version

```sh
lazy --version
```

The current development version comes from the checked-in `VERSION` file, which
is embedded into the binary at build time.

For app-bound commands, `lazy` reads the current module's `golazy.dev`
requirement from `go.mod`. If that framework version differs from the running
CLI version, `lazy` uses the matching CLI binary from:

```text
<user-cache-dir>/golazy/lazy/builds/<version>/lazy
```

When the cached binary is missing, `lazy` installs it with:

```sh
GOBIN=<user-cache-dir>/golazy/lazy/builds/<version> go install golazy.dev/lazy@<version>
```

The matching binary receives the same command-line arguments with
`LAZY_MULTIVERSION=off` in its environment, which prevents recursive version
handoffs. `lazy --version` and `lazy new` always use the binary that was
directly invoked.

Set `LAZY_MULTIVERSION=off` when you intentionally need the directly invoked
binary to run app-bound commands against a mismatched local application, for
example during CLI development and tests:

```sh
LAZY_MULTIVERSION=off lazy js
```

## Structure

- `main.go`: command dispatch and version output.
- `VERSION`: build version embedded into the binary.
- `version_handoff.go`: app framework version detection and CLI re-exec.
- `commands/run`: application discovery, hot reload, proxying, and execution.
- `commands/services`: mise service discovery and one-shot task execution.
- `commands/datasets`: dataset dump and load coordination.
- `commands/lazyconfig`: `lazy.toml` parsing.
- `commands/routes`: route-table inspection.
- `commands/upgrade`: one-step application upgrades and migration helpers.
- `commands/lazycode`: Go source rewrite helpers used by upgrade migrations.
- `commands/js`: JavaScript library bundling, app-module hashing, directive
  expansion, and importmap generation.
- `commands/tailwind`: Tailwind CLI setup and stylesheet compilation.
- `services/lifecycleservice`: managed local service subprocess lifecycle.
- `commands/appcmd`: shared application command discovery.
- `commands/new`: tagged template cloning, renaming, and validation.
- `commands`: shared subprocess execution.

## Build

```sh
go build .
```

## License

The GoLazy CLI is released under the MIT License. See [LICENSE](LICENSE).
