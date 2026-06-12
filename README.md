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

## Create an application

```sh
lazy new github.com/guillermo/my_app
```

The command creates `./my_app` from the `golazy/sample_app` tag matching the
CLI version, removes the template Git history, changes the module and imports,
then runs `go mod tidy` and `go test ./...`.

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
- `commands/new`: tagged template cloning, renaming, and validation.
- `commands`: shared subprocess execution.

## Build

```sh
go build .
```
