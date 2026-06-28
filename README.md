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
a small reload client into HTML responses. Proxy startup, generated-asset work,
builds, and app starts use compact progress lines. Successful task output stays
hidden, failed task output is printed, and the application process still writes
directly to the terminal. If the app is not running, the proxy serves a status
page with the latest build state.

When an application has `lazy.toml`, `lazy` opens the configured development
workspace in tmux through mise. Service panes run `mise run <service>:start`,
runner panes run their configured commands, the app pane runs the normal
development loop, and a `lazy command-center` pane stays open for local
inspection. The app pane receives `LAZY_TMUX=1` so it runs the development loop
instead of opening another tmux session.

Example `lazy.toml`:

```toml
services = ["postgres", "s3"]

[tmux]
session = "my-app"

[[runners]]
name = "tailwind"
command = "lazy tailwind --watch"

[[programs]]
name = "editor"
command = "nvim"
window = "workspace"
```

Each service is implemented as mise tasks named `{service}:{action}`. For a
stateful service such as PostgreSQL, define `postgres:start`,
`postgres:check`, `postgres:create`, `postgres:dump FILE`,
`postgres:load FILE`, and `postgres:migrate`. `postgres:start` is mandatory
and must run in the foreground so SIGINT stops it. `postgres:check` should
return status 0 only when the service is active and ready for dependent
processes; the `lazy` command may poll it before starting the app. Add
`postgres:kill` only as an escape hatch for stale local processes.

Use direct Go commands when you want to run without the development proxy or
watcher:

```sh
go run ./cmd/app
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
blocks, bundles those library entrypoints with esbuild, bundles application
modules from `app/js`, expands the `// golazy:turbo` and
`// golazy:stimulus` directives in `app/js/app.js`, and writes the importmap.
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
module and imports, trusts the generated `mise.toml`, runs `mise install`, then
validates with the current `go` on `PATH` by running `go mod tidy` and
`go test ./...`. After validation it initializes a fresh Git repository,
commits the generated checkout with a command-local GoLazy identity, and
prints the generated app directory and the `lazy` command to run next.

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
- `commands/lazyconfig`: `lazy.toml` parsing.
- `commands/lazytmux`: tmux session construction for configured workspaces.
- `commands/commandcenter`: interactive tmux command-center pane.
- `commands/routes`: route-table inspection.
- `commands/upgrade`: one-step application upgrades and migration helpers.
- `commands/lazycode`: Go source rewrite helpers used by upgrade migrations.
- `commands/js`: JavaScript library and app-module bundling, directive
  expansion, and importmap generation.
- `commands/tailwind`: Tailwind CLI setup and stylesheet compilation.
- `commands/appcmd`: shared application command discovery.
- `commands/new`: tagged template cloning, renaming, and validation.
- `commands`: shared subprocess execution.

## Build

```sh
go build .
```

## License

The GoLazy CLI is released under the MIT License. See [LICENSE](LICENSE).
