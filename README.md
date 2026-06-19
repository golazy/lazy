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

`lazy` is a development runner. It builds the app command into a temporary
binary, serves the public `ADDR` or `PORT` through a local proxy, runs the app
on an internal loopback port, watches application files, rebuilds and restarts
on changes, and injects a small reload client into HTML responses. Build output
is printed to stderr; if the app is not running, the proxy serves a status page
with the latest build state.

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
lets `lazyapp.New` initialize the app and call `Draw`, then prints the route
table without starting the HTTP server.

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
```

The command creates `./my_app` from the `golazy/sample_app` tag matching the
CLI version, removes the template Git history, changes the module and imports,
then runs `go mod tidy` and `go test ./...`.

For local validation against the checked-out sample application, point `lazy
new` at a directory:

```sh
lazy new --source-dir ../sample_app github.com/guillermo/my_app
```

## Version

```sh
lazy --version
```

The current development version comes from the checked-in `VERSION` file, which
is embedded into the binary at build time.

## Structure

- `main.go`: command dispatch and version output.
- `VERSION`: build version embedded into the binary.
- `commands/run`: application discovery, hot reload, proxying, and execution.
- `commands/routes`: route-table inspection.
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
