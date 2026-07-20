# Testing And Verification

Use the smallest test boundary that catches the behavior.

## Focused Package Tests

Put service tests beside the service:

```text
services/<name>/<name>_test.go
```

Use these for deterministic business behavior, parsing, validation, repository
logic with fakes, and service-specific edge cases.

When `New` uses the `lazydeps` lifecycle shape, assert construction errors and
call a non-nil cleanup function in the test. Add focused cleanup tests for
services that own goroutines, pools, files, listeners, or clients.

## Application HTTP Tests

Put full app tests under:

```text
test/
```

Use `lazytest` with the real app:

```go
func TestPosts(t *testing.T) {
	app := lazytest.New(t, appinit.App())

	app.Get("/posts").
		OK().
		ContentType("text/html").
		Contains("Posts")
}
```

This verifies the real composition path: dependencies, route registration,
controller construction, action execution, rendering, layout composition,
public assets, sessions, redirects, method errors, and route helpers.

## What To Assert

For routed HTML pages, assert:

- Status code and content type.
- Expected body content.
- Missing-record behavior when relevant.
- Redirect target and status when relevant.
- Method errors and `Allow` headers for unsupported methods.

For assets, assert:

- Logical public paths work.
- Rendered pages use permanent hashed asset paths when expected.
- Permanent asset paths set immutable cache headers.
- Importmaps contain stable browser import names.

For routes, assert:

- `lazy routes` prints the route you expect.
- The route table has only the new public surface the feature should expose.
- Path parameters use the expected names.

## Verification Commands

Run the relevant commands from the application root:

```sh
lazy routes
go test ./...
go test -race ./...
go vet ./...
```

When browser assets changed, run the generators before Go tests and builds:

```sh
lazy tailwind
lazy js
go test ./...
```

When building an executable, choose an output path outside the `app/`
directory:

```sh
mkdir -p .tmp
go build -o .tmp/sample-app ./cmd/app
```

## Related

[Services](services.md) | [Routes](routes.md) | [Controllers](controllers.md) |
[Assets](assets.md)
