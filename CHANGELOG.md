# Changelog

All notable changes to the GoLazy CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the CLI uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/golazy/lazy/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/golazy/lazy/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/golazy/lazy/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/golazy/lazy/releases/tag/v0.1.1
