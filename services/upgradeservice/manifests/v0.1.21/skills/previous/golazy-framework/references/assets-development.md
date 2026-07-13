# Assets And Development Loop

GoLazy serves public files from the application binary in production and from
the local filesystem in `lazydev` builds.

## Embedded Files

`app/files.go` embeds the conventional application files:

```go
//go:embed views public
var Files embed.FS

var Views = lazyapp.MustSub(Files, "views")
var Public = lazyapp.MustSub(Files, "public")
```

`init/app.go` passes `app.Views` and `app.Public` to `lazyapp.New`.

## Public Assets

Files under `app/public` are served by `lazyassets`.

- Logical paths look like `/styles.css`.
- Permanent hashed paths look like `/styles-<hash>.css`.
- Templates should link logical paths through helpers. Helpers return permanent
  paths in production when a permanent path exists.

Use:

```gotemplate
{{stylesheet "/styles.css"}}
{{importmap "/assets/importmap.json"}}
<script type="module">import "app.js"</script>
```

## Tailwind

Tailwind source lives in:

```text
app/styles/application.css
```

The generated public stylesheet lives in:

```text
app/public/styles.css
```

Run this when Tailwind source, package files, or CSS dependencies change:

```sh
lazy tailwind
```

Run `lazy tailwind --watch` in a separate terminal during focused UI work.

## JavaScript

App-owned JavaScript source lives in:

```text
app/js/app.js
app/js/controllers/
```

`js.toml` declares library entrypoints for `lazy js`. The command writes:

```text
app/public/assets/importmap.json
app/public/assets/lazyshaft/
```

Run this when `js.toml`, package files, JavaScript library versions, or files
under `app/js` change:

```sh
lazy js
```

Commit generated importmaps and lazyshaft outputs. Do not commit
`node_modules`.

## `lazy` Development Behavior

Run the development loop from the app root:

```sh
lazy
```

The CLI builds a `lazydev` binary, starts the app behind the local proxy,
serves the development panel at `/_golazy/`, runs `lazy js` automatically when
the app has `js.toml`, watches files, reloads views from disk, reloads public
assets, rebuilds as needed, and stops cleanly on Ctrl-C.

Use:

```sh
ADDR=127.0.0.1:4000 lazy
PORT=4000 lazy
```

to change the browser-facing address.

## Service Tasks And Datasets

If the app needs local services, define task files under `.mise/tasks/<service>/`:

```text
<service>:start
<service>:check
<service>:create
<service>:migrate
<service>:dump
<service>:load
```

`lazy` starts and prepares services before the app. `lazy dump <name>` and
`lazy load <name>` pass paths under `datasets/<name>/<service>.dump` to the
service tasks.
