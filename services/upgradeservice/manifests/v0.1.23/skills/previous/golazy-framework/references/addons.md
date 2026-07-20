# Add-ons

Use add-ons when an application needs a versioned capability that owns both
installation changes and typed runtime behavior.

## Commands

Run commands at the application root:

```sh
lazy add postgres
lazy add --config database_url_variable=APP_DATABASE_URL postgres
lazy add postgres/jobs
lazy add --dry-run postgres
lazy addons
lazy update postgres
lazy remove postgres/jobs
```

Repeat `--config key=value` to merge committed, non-secret runtime settings;
use `--unset-config key` to remove one. These values are stored in
`addons.toml` and generated into `lazyaddon.Use.Config`, so secrets belong in
the referenced environment variable rather than the lock file.

`addons.toml` is the committed desired state and exact lock. It records direct
and transitive selections, package and module versions, manifest digests, and
managed-file baselines, plus non-secret configuration. Do not hand-edit generated add-on files; a hash
mismatch intentionally blocks update or removal.

Same-package requirements inherit the package's exact version. A dependency
on another package uses `id@vX.Y.Z`; incompatible requirements fail before any
file is written. Runtime wiring also checks every locked version against the
compiled registrations.

Use `--manifest <path>/lazyaddon.toml` while developing a local add-on. The
installer applies declarative files, JavaScript entries, tasks, and structured
source changes through one `lazycode` plan. It does not run arbitrary install
code from the package.

Portable files use `source=>target`. JavaScript libraries use
`package@exact-version`, entrypoints use `name=module`, and build hooks use
`before-build=<task>` or `after-build=<task>` with a same-add-on task installed
under `.mise/tasks`. `source_edits = ["edits/addon.json"]` supports schema-1
`toml.set_string.v1`, `toml.set_strings.v1`, and `go.ensure_import.v1`
operations. Descriptor bytes are digest-bound; property ownership makes edits
shareable and reversible and stops removal on drift.

Installed Go packages register typed `lazyaddon` hooks for files,
dependencies, migrations, jobs, routes, helpers, and the lazydev control
plane. App files remain the highest `lazyfs` layer. Dependencies run before
their dependents, and add-on-specific shared values use typed capabilities.
Panel functionality uses named actions backed by exact, same-owner, POST-only
endpoints; the trusted host resolves IDs and sends an empty loopback POST.

An add-on that extends the app base controller contributes a generated sidecar
such as `app/controllers/seo_addon.go`; Go cannot attach methods from the add-on
package. Never use `_seo.go`, because Go ignores underscore-prefixed source
files.

The base `postgres` add-on owns the pgx pool and migration backend.
`postgres/jobs` is separate: it requires the base, mounts `pgjobs` migrations,
and supplies the durable jobs backend. App migrations live under
`db/postgres/migrations`.

SEE OTHERS: [Services](services.md), [Jobs](jobs.md), [Js](js.md),
[Controllers/Base](controllers-base.md), [Views](views.md)
