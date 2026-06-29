# Changelog

All notable changes to the GoLazy CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the CLI uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `lazy dump <dataset>` and `lazy load <dataset>` coordinate service
  `dump` / `load` mise tasks with files under `datasets/<dataset>`.
- `lazy new` now renames the generated command directory from `cmd/app` to
  `cmd/<app-name>` and rewrites template docs and build commands to match.
- The development panel Routes tab now reads the application's lazydev route
  table and renders a filterable route list.
- The development panel now includes a Jobs tab that proxies the application's
  lazyjobs control-plane state.
- The development panel now includes a Services tab with per-service output and
  status-bar service indicators.
- The development panel Services tab now includes per-service restart actions.
- The development panel Services tab now splits service output by lifecycle
  script (`start`, `check`, `create`, `migrate`, and custom task names) and
  shows each task run number, making repeated readiness checks visible.
- The development panel now includes an App tab with service status, lazy
  lifecycle events, changed-file groups, and manual rebuild, restart, and
  open-app controls.
- The development panel Assets tab now lists lazy asset manifest entries and
  their public paths from the app lazydev control plane.
- The development panel Cache tab now shows cache size, usage, hit/miss/set
  counters, a searchable key table with age and size, and selected cache entry
  content when the app backend exposes inspectable entries.
- The `lazy` development proxy now accepts HTTP and HTTPS on the same port.
  Plain HTTP serves the local certificate authority setup and download page,
  while HTTPS serves the development panel and proxied app traffic with HTTP/2
  available after the browser trusts the generated local CA.
- The `lazy` development proxy now serves `/_golazy/extension` as a tiny
  Chrome DevTools extension handshake. The extension probes that endpoint and
  embeds the site's `/_golazy/` panel when the exact `i love being lazy` body
  is returned.
- The `lazy` development proxy now serves Chromium Automatic Workspace Folders
  metadata at `/.well-known/appspecific/com.chrome.devtools.json`, pointing
  Chrome DevTools at the app's `app/js` source folder.
- The injected development-panel host script now exposes
  `window.disableDevPanel()` so the Chrome DevTools extension can hide the
  in-page panel and launcher once the inspected page has loaded.
- `lazy upgrade` now migrates `v0.1.16 -> v0.1.17` controller calls from
  `SetLayout` to `Layout` and rewrites common `CacheKey` / `CacheKeyF` action
  returns for the new boolean cache-hit contract.

### Changed

- The default `lazy` development command no longer forces OTEL trace/log
  exporters into the child app. Detailed request monitoring is off by default
  and can be enabled from the development panel.
- The development panel now visits one resource-backed page per top-level tab,
  keeps the status bar mounted as a permanent Turbo Frame, and uses one
  permanent status `turbo-stream-source` plus one tab-scoped stream source for
  fresh data. Browser-facing panel endpoints render HTML, Turbo Frames, or
  Turbo Stream HTML; internal app-control JSON is read server-side and enriched
  before rendering.
- Development panel tab streams now hydrate list content when their
  `turbo-stream-source` connects, then send targeted row/count updates only
  when a relevant backend event exists instead of repainting whole tabs from
  generic build events.
- The development panel Requests tab now reads captured request sidecars,
  lists request paths, and exposes Headers, Tracing, and Logs detail tabs.
  The Tracing detail renders per-region total and self duration plus sampled
  allocation bytes, malloc counts, and free counts when the app sidecar
  provides lazydev allocation samples.
- The development panel Requests tab now filters request paths and request
  categories on the app control plane, refreshes new matching requests through
  its tab stream, clears trace sidecars from the clear button, and lazy-loads
  request details in a nested Turbo frame.
- The Requests Tracing detail now renders a status strip, an Include golazy
  toggle, a backend-sorted region metrics table, and a flamegraph whose scale
  follows the selected time, allocation, or memory metric.
- The Chrome extension action now toggles the inspected page's in-page panel
  instead of trying to select the DevTools panel. The in-page panel hides when
  the GoLazy DevTools panel is open, and closed panels show a small yellow
  GoLazy launcher only when the extension is not installed.
- The development iframe client now relies on Turbo Stream sources and
  Stimulus controllers for panel UI behavior, removing the old shared
  `/_golazy/state` fetch loop and `/_golazy/events` panel renderer while
  keeping the host-page reload stream.
- The development panel now uses a single top tab bar with the close control on
  the right, app and service status chips in the permanent status bar, an App
  tab for app lifecycle state, and a full-height Services page.
- The status bar app chip now opens App, and selected service chips keep
  the same background as the rest of the status bar.
- The development panel no longer renders Console, App Logs, or Actions tabs.
  App logs are merged into Services through a synthetic App service, and Cache
  owns the cache controls and cache inspection table.
- `lazy docs --json` includes package, value, function, type, and method source
  file and line metadata from `golazy.dev/lazydoc`.
- The default `lazy` command now discovers local services from `lazy.toml` or
  `:start` mise tasks, starts them as managed subprocesses in parallel, records
  their stdout and stderr, waits on service `check` tasks before running
  `create` and `migrate`, and stops the app before stopping services on
  interrupt.
- Service lifecycle output events now keep the service name, script action, and
  run number so repeated `check` attempts and restart-triggered runs can be
  inspected independently in the panel.
- Managed service rows in the Services tab now expose restart, stop, and start
  actions, while the service tree lists discovered lifecycle scripts and
  separates general mise tasks below the services.
- Service output in the development panel is capped at 100 rendered rows,
  batched while streaming, and parses JSON log lines into message and
  attributes columns.
- The embedded development panel can now be closed from its toolbar. Panel
  split panes use a reusable Stimulus resize controller with `left`, `right`,
  `top`, and `bottom` directions plus pixel or percentage `min`, `max`, and
  `size` values.
- Development panel data tables now use a reusable Stimulus column-resize
  controller, with per-header minimum widths and resize handles across the
  panel's shared table component. Tables remember resized widths in browser
  local storage, and rightward drags now continue compressing later columns
  after the immediate right column reaches its minimum.
- Embedded development panels can now be resized from their top edge, and the
  proxied page's bottom padding follows the selected panel height.
- `lazy js` and `lazy tailwind` now choose Node package managers from active
  installed mise tools in `pnpm`, `yarn`, `bun`, `node` order. Managers found
  through mise run with `mise exec`; apps without a usable mise package-manager
  tool fall back to direct `npm` / `npx`.
- `lazy js` now keeps app-owned files under `app/js` readable and unbundled
  while still writing content-hashed lazyshaft assets and importmap entries.
  Manifest-declared library entrypoints remain bundled.
- `lazy js` now writes app-owned importmap specifiers relative to `app/js`, so
  layouts import `app.js` and generated Stimulus wiring imports controllers as
  `controllers/<name>_controller.js` instead of `/js/...` URL paths.

### Fixed

- The development panel Traces tab now stops parent-depth walks when span data
  contains cyclic parent links, preventing malformed `.spans` sidecars from
  freezing the browser.

## [0.1.16] - 2026-06-27

### Added

- The development panel now renders Requests, Console, App Logs, Traces,
  Routes, Assets, and Actions tabs in a DevTools-style shell, with the existing
  build/run output, event stream, cache controls, rebuild, restart, and open-app
  actions carried into that shell.
- `lazy upgrade` now recognizes `v0.1.16` and advances `v0.1.15` applications
  by updating their `golazy.dev` module requirement through the versioned
  `go.mod` manifest.

### Changed

- Proxied app pages now embed the GoLazy development panel as a fixed bottom
  iframe that survives Turbo navigation instead of showing only a floating
  activator button.
- The `lazy` development proxy now forwards request IDs and trace context to
  the child app so lazydev request artifacts line up with framework telemetry.

## [0.1.15] - 2026-06-25

### Added

- `lazy upgrade` now migrates `v0.1.14 -> v0.1.15` application initializers by
  renaming `init/context.go` to `init/dependencies.go`, rewriting
  `lazyapp.Config.Context` to `lazyapp.Config.Dependencies`, and updating the
  initializer signature. It also moves inline `lazyapp.Config.SEO` option
  slices into `init/seo.go` and wires `SEO: SEO`.
- `lazy upgrade` file manifests now track sample-app additions, replacements,
  and removals with SHA-256 hashes. Unchanged files are updated or removed
  directly, new files are created directly, and customized conflicts prompt for
  dated backups or explicit keep decisions.
- `commands/lazycode` provides shared Go source rewrite helpers for upgrade
  migrations using `go/parser`, AST edits, `go/format`, and changed-file
  writes.

### Changed

- `LAZY_MULTIVERSION=off` now disables project-version CLI handoff for local
  testing, replacing the removed global `--skip-version-check` flag.
- The default `lazy` development command now skips automatic `go mod tidy` when
  `GOWORK` or `go env GOWORK` points at an active Go workspace.
- CLI-owned environment variables are now loaded once into the package-level
  `Config` singleton in `lazy/config.go` using
  `golazy.dev/lazyconfig.MustGetenv` instead of being read ad hoc throughout
  the command tree.
- The default `lazy` development command now uses `golazy.dev/lazytui/progress`
  for proxy startup, generated-asset work, Go builds, and application starts
  while leaving the running app's own output attached to the terminal.
- `lazy upgrade` now applies versioned `go.mod` requirement manifests through
  `go get` instead of rewriting `go.mod` directly. Its `mise.toml` manifests
  add or update required tools and comment obsolete tools or task tables with a
  reason instead of silently deleting them.
- `lazy new` now validates generated apps in workspace mode with
  `go work sync`, and generated app mise manifests pin helper tool versions
  instead of using `latest`.

## [0.1.14] - 2026-06-23

### Added

- `lazy new --version <version>` can generate an app from a specific
  `golazy/sample_app` tag.

### Changed

- `lazy new` now checks `https://golazy.dev/lazy.version` before cloning a
  remote template and stops when a newer CLI is available. The check has a
  one-second timeout, ignores network failures, and can be skipped with
  `--skip-update-check`.
- `lazy new` now falls back to `$HOME/.local/bin/mise` for generated-app setup
  when `mise` was installed but the current shell has not loaded the updated
  `PATH` yet.
- `lazy new` now initializes a fresh Git repository and commits the generated
  checkout after validation succeeds, using a command-local GoLazy identity so
  fresh systems without global Git author config still work.
- `lazy new` now prints concrete next steps with the generated app directory
  and `lazy` command.
- `lazy new`, `lazy`, `lazy routes`, `lazy upgrade`, and the app build step of
  `lazy native build` keep Go subprocesses on the current `go` from `PATH`,
  while `lazy js` and `lazy tailwind` run app-managed package-manager tools
  through `mise exec`.
- The default `lazy` development command now runs `go mod tidy` before building
  or running the app, so module files are checked and repaired as part of the
  dev loop.
- `lazy upgrade` now uses the framework progress UI for clearer task status
  while still allowing interactive conflict and prompt output to take over the
  terminal deliberately.
- Updated the CLI release version to `v0.1.14` so `lazy new` selects the
  matching sample application template once the coordinated release is
  published.

## [0.1.13] - 2026-06-22

### Added

- `lazy native` opens the current application through the external native
  desktop helper while keeping the normal development loop in charge of
  rebuilds and reloads.
- `lazy native build` builds the current application for the host platform and
  passes the server binary to the native helper for packaging.
- `lazy upgrade` starts the application-upgrade workflow. It reads the current
  app's `golazy.dev` requirement, can advance one release or a `--target`
  range, applies the backfilled `v0.1.10 -> v0.1.11` and `v0.1.11 -> v0.1.12`
  migrations, advances `v0.1.12 -> v0.1.13` by updating `go.mod`, supports
  `--force <version>` for rerunning a specific one-step migration, reports
  customized-file conflicts with diffs, and runs `go mod tidy`, `go test
  ./...`, and `go vet ./...` after each step.

### Changed

- The CLI module path moved to `golazy.dev/lazy`; version handoff and install
  documentation now use the vanity import path while the repository remains
  `github.com/golazy/lazy`.
- `lazy new` now trusts the generated app's `mise.toml`, runs `mise install`,
  and validates with the current `go` on `PATH` so apps can omit Go from
  `mise.toml` while still using mise for project tools.
- Generated app `mise.toml` files no longer list Go as a mise tool. Go already
  bundles multi-version support through the module `go` directive and
  toolchain selection, and `lazy` prompts to remove stale app-level Go entries.
- Updated the CLI release version to `v0.1.13` so `lazy new` selects the
  matching sample application template once the coordinated release is
  published.

## [0.1.12] - 2026-06-22

### Added

- `lazy` now reads optional `lazy.toml` workspace configuration. When present,
  the default `lazy` command opens a tmux development session through mise with
  service panes, runner panes, the app development loop, and `lazy
  command-center`.
- `lazy` now checks the current app module's `golazy.dev` requirement before
  app-bound commands. When the framework version differs from the running CLI
  version, it runs or installs the matching `github.com/golazy/lazy` version
  under the user cache and re-executes the command with version checking
  disabled.
- `lazy --skip-version-check` lets CLI development and test runs keep using the
  directly invoked binary even when the app requires a different framework
  version.
- `lazy docs` can inspect local Go package documentation and print package
  summaries, search results, or JSON using the shared `golazy.dev/lazydoc`
  model.
- `lazy command-center` provides the first interactive pane for tmux workspaces.

### Changed

- `lazy` now resolves local development view and public paths itself and passes
  the concrete paths to `lazydev` application builds through linker flags.
- Updated the CLI release version to `v0.1.12` so `lazy new` selects the
  matching `golazy/sample_app` template tag with top-level application services.

## [0.1.11] - 2026-06-21

### Changed

- Updated the CLI release version to `v0.1.11` so `lazy new` selects the
  matching `golazy/sample_app` template tag with SEO metadata setup and
  standalone mise task scripts.

### Fixed

- `lazy` hot reload no longer injects the reload client into Turbo Frame
  requests or HTML fragments that do not include a document body.

## [0.1.10] - 2026-06-20

### Changed

- Updated the CLI release version to `v0.1.10` so `lazy new` selects the
  matching `golazy/sample_app` template tag with `lazytest` integration,
  secure-cookie environment setup, Docker packaging, and the latest controller
  route/form helpers.

### Fixed

- `lazy` hot reload now fingerprints JavaScript package metadata before
  rebuilding, avoiding restart loops when package managers rewrite lockfile
  timestamps without changing file content.

## [0.1.9] - 2026-06-19

### Added

- `lazy js` now bundles application JavaScript from `app/js`, writes `/js/...`
  importmap entries for every app JavaScript file, and expands
  `// golazy:turbo` and `// golazy:stimulus` directives in `app/js/app.js`.
- `lazy` hot reload now runs JavaScript asset generation for apps with
  `js.toml` before the initial build and after changes to `app/js`,
  `js.toml`, or JavaScript package metadata.

### Changed

- Updated the CLI release version to `v0.1.9` so `lazy new` selects the
  matching `golazy/sample_app` template tag with app-owned JavaScript modules,
  controller formats, redirects, response metadata helpers, and SSE examples.

## [0.1.8] - 2026-06-19

### Added

- `lazy tailwind` initializes Tailwind input stylesheets, installs Tailwind CLI
  dependencies, and compiles CSS into embedded public stylesheets for
  conventional and single-file GoLazy applications.
- `lazy` now runs applications through a hot-reload development loop: it builds
  a temporary binary, watches application files, restarts the app after
  successful rebuilds, keeps the previous app process during failed rebuilds,
  and injects a browser reload client into HTML responses.

### Changed

- `lazy routes` shows namespaced route targets so route tables distinguish
  controllers such as `admin/posts#index`.
- Updated the CLI release version to `v0.1.8` so `lazy new` selects the
  matching `golazy/sample_app` template tag with Tailwind, dark-mode sample
  styles, action generators, route namespaces, and hot reload documentation.

## [0.1.7] - 2026-06-17

### Added

- `lazy js` to install JavaScript package dependencies declared in `js.toml`,
  bundle library entrypoints with esbuild, copy declared assets, and write the
  generated importmap used by application browser modules.
- `--cmdpath` and `--viewpath` flags for `lazy` and `lazy routes`, allowing
  applications to choose a command entrypoint and local view directory during
  development.

### Changed

- Application command discovery now scans `./cmd` for main packages that import
  `golazy.dev/lazyapp` instead of only trying `./cmd/<module-name>` and
  `./cmd/app`.
- Updated the CLI release version to `v0.1.7` so `lazy new` selects the
  matching `golazy/sample_app` template tag with JavaScript library assets and
  form helpers.

### Fixed

- `lazy new --source-dir` now skips `node_modules` when copying local sample
  app templates.

## [0.1.6] - 2026-06-17

### Changed

- Updated the CLI release version to `v0.1.6` so `lazy new` selects the
  matching `golazy/sample_app` template tag with sessions, server helpers, and
  pooled-controller conventions.

### Fixed

- `lazy new` now replaces the sample app's session key with fresh random
  16-character hex key material in generated applications.

## [0.1.5] - 2026-06-16

### Changed

- Updated the CLI release version to `v0.1.5` so `lazy new` selects the
  matching `golazy/sample_app` template tag with asset permalink support.

## [0.1.4] - 2026-06-15

### Changed

- `lazy` and `lazy routes` now run applications with the `lazydev` build tag,
  so development commands use local disk views while production builds keep
  embedded views.
- `lazy new --source-dir` validates generated applications with temporary
  workspace replacements when preparing an unpublished framework release,
  without leaving local `replace` directives in the generated app.
- Updated the CLI release version to `v0.1.4` so `lazy new` selects the
  matching `golazy/sample_app` template tag.

## [0.1.3] - 2026-06-15

### Added

- `lazy routes` to inspect a GoLazy application's route table without starting
  the HTTP server.

### Changed

- Shared application command discovery between `lazy` and `lazy routes`, using
  `./cmd/<module-name>` first and falling back to `./cmd/app`.
- Updated the CLI release version to `v0.1.3` so `lazy new` selects the matching
  `golazy/sample_app` template tag.

## [0.1.2] - 2026-06-12

### Changed

- Updated the CLI release version to `v0.1.2` so `lazy new` selects the
  matching `golazy/sample_app` template tag.

## [0.1.1] - 2026-06-12

### Added

- `lazy` runs the current application from `cmd/<module-name>` or `cmd/app`.
- `lazy new <module>` creates an application from the matching tagged
  `golazy/sample_app` release.
- `lazy --version` reports the CLI version used to select the application
  template.

### Changed

- The CLI version now comes from the checked-in `VERSION` file embedded into
  the binary at build time.

[Unreleased]: https://github.com/golazy/lazy/compare/v0.1.16...HEAD
[0.1.16]: https://github.com/golazy/lazy/compare/v0.1.15...v0.1.16
[0.1.15]: https://github.com/golazy/lazy/compare/v0.1.14...v0.1.15
[0.1.14]: https://github.com/golazy/lazy/compare/v0.1.13...v0.1.14
[0.1.13]: https://github.com/golazy/lazy/compare/v0.1.12...v0.1.13
[0.1.12]: https://github.com/golazy/lazy/compare/v0.1.11...v0.1.12
[0.1.11]: https://github.com/golazy/lazy/compare/v0.1.10...v0.1.11
[0.1.10]: https://github.com/golazy/lazy/compare/v0.1.9...v0.1.10
[0.1.9]: https://github.com/golazy/lazy/compare/v0.1.8...v0.1.9
[0.1.8]: https://github.com/golazy/lazy/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/golazy/lazy/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/golazy/lazy/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/golazy/lazy/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/golazy/lazy/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/golazy/lazy/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/golazy/lazy/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/golazy/lazy/releases/tag/v0.1.1
