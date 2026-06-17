# Changelog

All notable changes to the GoLazy CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the CLI uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/golazy/lazy/compare/v0.1.7...HEAD
[0.1.7]: https://github.com/golazy/lazy/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/golazy/lazy/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/golazy/lazy/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/golazy/lazy/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/golazy/lazy/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/golazy/lazy/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/golazy/lazy/releases/tag/v0.1.1
