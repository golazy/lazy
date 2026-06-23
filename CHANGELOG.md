# Changelog

All notable changes to the GoLazy CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the CLI uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `lazy new` now initializes a fresh Git repository and commits the generated
  checkout after validation succeeds.

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

- `lazy` now resolves local development view paths itself and passes the
  concrete path to `lazydev` application processes through `GOLAZY_VIEW_PATH`,
  instead of configuring framework view lookup through linker flags.
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

[Unreleased]: https://github.com/golazy/lazy/compare/v0.1.13...HEAD
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
