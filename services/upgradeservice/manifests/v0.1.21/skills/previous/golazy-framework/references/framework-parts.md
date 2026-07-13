# Framework Parts

GoLazy is split into small packages. Most application work uses `lazyapp` as
the composition layer, then relies on a few focused packages indirectly.

## Core Request Path

- `lazyapp`: application composition. It initializes dependencies, views,
  public assets, route scopes, controllers, sessions, jobs, cache, optional
  SEO/PWA/workers/media wiring, and the final HTTP handler.
- `lazyroutes`: route table, named path helpers, resource routes, route
  metadata, namespaces, and controller action binding.
- `lazycontroller`: request-local controller base, render state, layouts,
  response status and headers, redirects, content negotiation, expected HTTP
  errors, cache keys, deferred view values, and SSE entrypoints.
- `lazyview`: renderer abstraction, template helpers, layouts, partials, and
  trusted fragments. The sample app imports `golazy.dev/lazyview/gotmpl` to
  register Go `html/template` files.
- `lazydispatch`: framework dispatch plumbing used by `lazyapp` to route
  requests through controllers, errors, middleware, and fallbacks.

## Application Services

- `lazydeps`: initializes long-lived services once, records dependency edges,
  stores the resulting app context, and shuts services down in dependency-safe
  order.
- `lazyconfig`: fills typed config structs from environment variables.
- `lazyerrors`: creates errors with caller context and backtraces for useful
  development detail pages.
- `lazytest`: HTTP-level assertions against the real app handler without
  opening a port.

## Assets And Templates

- `lazyassets`: registers embedded public files, computes content hashes,
  serves logical and permanent asset paths, rewrites CSS `url(...)` references,
  and provides `asset_path` and `stylesheet` helpers.
- `lazyforms`: model-aware form helpers for templates. It renders forms; request
  parsing still belongs in controller actions or services.
- `lazyturbo`: Turbo helpers and response support for server-rendered updates.
- `lazyseo`: SEO, metadata, sitemap, and robots helpers.
- `lazypwa` and `lazyworkers`: opt-in progressive web app and browser worker
  registration.

## State, Messaging, And Storage

- `lazyauth`: authentication contracts and app auth context. `lazyapp.New`
  includes it by default with an in-memory backend that has zero users unless
  `LAZYAUTH_DEFAULT_PASS` creates a bootstrap `admin` user. Set
  `LAZYAUTH_DEFAULT_USER` to use a different username, or provide
  `lazyapp.Config.Auth` for file, PostgreSQL, SSO, or app-specific auth.
- `lazysession`: signed cookie or custom-store sessions plus flash messages.
  Enable it through `lazyapp.Config.Sessions`.
- `lazymailer`: renders email views with `lazyview` and sends MIME messages
  through SMTP, memory, or custom deliveries.
- `lazyjobs`: in-process and durable job definitions and execution.
- `lazyfiles`, `lazymedia`, and `lazystorage`: file catalogs, media variants,
  and object storage abstractions.
- `lazycache`: cache primitives used by render cache helpers and app services.

## Databases And Migrations

- `lazymigrate`: backend-agnostic migration loading, planning, and execution.
  It does not parse SQL itself.
- `golazy.dev/pg`: PostgreSQL implementations for framework backends, including
  pgx pool helpers and migration execution for SQL files with `-- +lazy Up`
  and `-- +lazy Down` sections.

## Realtime And Control

- `lazysse`: Server-Sent Events helpers.
- `lazywebsocket`: WebSocket support for apps and lazy-owned development
  tooling.
- `lazycontrolplane`: private control-plane routing for development and
  operational endpoints.

Use `lazy docs <package>` for exact package documentation from the current
module graph before depending on a newer or less common API.
