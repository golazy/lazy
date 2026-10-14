---
name: golazy-upgrade
description: Upgrade a GoLazy application safely across released framework versions. Use when moving golazy.dev to a newer version, running lazy upgrade, resolving an upgrade conflict or changed application pattern, validating each intermediate release, or committing a multi-version application migration.
---

# Upgrade A GoLazy Application

Move through one released GoLazy version at a time. Finish, verify, and commit
the current version before starting the next. Never combine several version
changes into one migration or one commit, even when the final target is known.

Use the sibling [GoLazy framework skill](../golazy-framework/SKILL.md) for the
application patterns synchronized by each upgrade.

## Establish The Baseline

1. Read `AGENTS.md`, `README.md`, and project-specific test or deployment docs.
2. Inspect `git status --short`. Start from a clean worktree. Do not mix
   unrelated edits into an upgrade commit; finish them separately or ask the
   user how to isolate them.
3. Read the current `golazy.dev` requirement from `go.mod` and record the final
   requested target. Do not infer the current version from an installed CLI.
4. Discover every application migration path before changing the version.
   Read `AGENTS.md`, `README.md`, `lazy.toml`, `.mise/tasks/*/migrate`, database
   documentation, and `init/` wiring such as `LAZYAPP_MIGRATE`. Use the app's
   documented command; do not invent a second migration mechanism.
5. Run all pending migrations for the current GoLazy version against the
   local or designated upgrade-test database. This clears migrations that may
   have been added by an earlier version but never applied. Never migrate a
   production database without explicit authorization and its documented
   backup or rollout procedure.
6. If the app has no migration mechanism, record `Migrations before <version>:
   no application migration command configured`. If the mechanism exists but
   reports no pending work, record `Migrations before <version>: no pending
   migrations`. Do not silently treat an unavailable database or failed command
   as either case.
7. Run the application's existing verification commands before changing the
   version. At minimum run `go test ./...` and `go vet ./...`; also run the
   app-specific integration, asset, race, build, or service checks required by
   `AGENTS.md`.
8. If the migration or baseline gate fails, record the failure and stop. Do
   not make an upgrade responsible for an unrelated broken baseline without
   explicit direction.

## Upgrade One Version

Repeat this loop until `go.mod` reaches the requested target:

1. Run `lazy upgrade --dry-run` with no `--target`. Confirm the plan names one
   step from the current version to its immediate successor.
2. Do not use `--target` to cross multiple versions. If the dry run skips
   releases because the app predates the automated upgrade path, stop and
   follow each historical release's upgrade notes rather than accepting a
   catch-up jump as a verified one-version migration.
3. Run `lazy upgrade` without `--skip-commands`. Allow it to update `go.mod`,
   apply the one-step migration, synchronize generated skills, and run its
   follow-up Go commands.
4. Inspect `git status --short` and the complete diff. Confirm that `go.mod`
   names only the intended next GoLazy version and that unrelated files were
   not changed.
5. Reread `.skills/golazy-framework/SKILL.md` and every synchronized reference
   changed by the step. Treat new conventions as the target pattern for source
   repairs in this version.
6. Resolve migration conflicts and failing checks before continuing. Prefer
   the new version's normal pattern over compatibility shims that preserve an
   obsolete application structure.
7. Run every documented application migration command again using the upgraded
   version. This applies migrations introduced by the target version and is a
   required gate before tests or the next version.
8. Record the target-side result as the command plus applied migrations,
   `Migrations after <version>: no pending migrations`, or `Migrations after
   <version>: no application migration command configured`.
9. Run the full verification gate below.
10. Commit only this version's dependency, migration, generated, and manual
   repair changes. Use a subject such as `upgrade: golazy v0.1.21 to v0.1.22`
   and include both before- and after-upgrade migration results in the commit
   body.
11. Confirm the commit exists and the worktree is clean before beginning the
   next loop.

Running `lazy upgrade` without a target intentionally advances one supported
release. Use `--force` only to diagnose or rerun the migration whose source
version is already understood; it does not authorize overwriting customized
files or skipping the normal loop.

## Repair A Version

Do not proceed to another version while any current-step migration, build, or
test is failing.

- Read the error and the generated diff before editing source.
- For `.golazy/upgrade/conflicts`, compare the current file with the proposed
  target. Preserve application-specific behavior while adopting the target
  convention; do not blindly replace customized files.
- If controller, service, dependency, asset, or initialization patterns
  changed, use the newly synchronized `golazy-framework` references to update
  all affected application code in this same version commit.
- Regenerate committed assets when their source or manifest changed. Never edit
  generated CSS, importmaps, or lazyshaft output by hand.
- Add or update focused tests when a manual repair changes behavior.
- Treat a migration failure as a version failure. Fix the migration, service
  readiness, configuration, or source compatibility and rerun it before tests.
  Do not continue merely because unit tests can run without the database.
- If the upgrade cannot be repaired safely, stop with the current version,
  exact failing command, diff summary, and unresolved decision. Do not skip the
  release or commit a knowingly broken step.

## Verify Every Version

Run the app's documented verification commands after the automated migration
and after every manual repair. Unless the app documents a stricter gate, run:

```sh
lazy routes
go test ./...
go test -race ./...
go vet ./...
mkdir -p .tmp
go build -o .tmp/app ./cmd/app
```

When `js.toml`, package files, `app/js`, Tailwind inputs, or their generated
outputs changed, run the relevant generators before Go tests:

```sh
lazy tailwind
lazy js
```

Run project-specific integration tests, database migrations, service checks,
or browser tests when the changed version affects those surfaces. A passing
package test is not enough when the application has a broader documented gate.

Before committing, run:

```sh
git diff --check
git status --short
git diff --cached
```

Stage only the current upgrade step. Include manual pattern repairs and their
tests in the same version commit because they are required to make that version
work.

## Finish

After the final version:

1. Confirm `go.mod` contains the requested GoLazy version.
2. Rerun the complete application verification gate once more.
3. Review the ordered upgrade commits with `git log --oneline`.
4. Confirm the worktree is clean, except for explicitly preserved unrelated
   user changes.
5. Report each version transition, its commit, any manual pattern migration,
   database migrations before and after the transition, and every verification
   command. Explicitly report when no migration command is configured or no
   migrations were pending. Report skipped or failing checks exactly.
