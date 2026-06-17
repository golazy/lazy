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

## Inspect routes

From a GoLazy application module:

```sh
lazy routes
```

The command runs the application with the `lazydev,printroutes` build tags,
lets `lazyapp.New` initialize the app and call `Draw`, then prints the route
table without starting the HTTP server.

## Build JavaScript libraries

From a GoLazy application module with `js.toml`:

```sh
lazy js
```

The command installs the JavaScript packages named by `[entrypoint.<name>]`
blocks, bundles those library entrypoints with esbuild, and writes an
importmap for app-owned browser modules. Application JavaScript is not bundled
by this command.

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
- `commands/run`: application discovery and execution.
- `commands/routes`: route-table inspection.
- `commands/js`: JavaScript library bundling and importmap generation.
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
