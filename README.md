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
- `commands/appcmd`: shared application command discovery.
- `commands/new`: tagged template cloning, renaming, and validation.
- `commands`: shared subprocess execution.

## Build

```sh
go build .
```

## License

The GoLazy CLI is released under the MIT License. See [LICENSE](LICENSE).
