# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

## [0.4.0] - 2026-10-01

 - Access control: an Authoriser per component; rendering takes a ctx

### Added
- Access control. `Explorer.Authoriser` (an `Authoriser` interface;
  `AuthoriserFunc` adapts a function) decides per component what the
  viewer, identified from the context, may see. Denied components drop out
  of the index, their pages say access is denied, and foreign keys into
  them show the value and owner without a link. Unowned tables are asked
  about as component `""`. View models gain `Denied` and `NoAccess`.

### Changed
- **Breaking:** `Render`, `IndexHTML`, `TableHTML` and `TableHTMLWith` take
  a `context.Context` first, carrying the viewer to the Authoriser.
  `Handler` passes the request's context.

## [0.3.0] - 2026-10-01

 - Skins and components: templates over view models, per-component scopes

### Added
- Skins. Pages render through `html/template`s over exported view models
  (`IndexView`, `TableView`, `PageView` and their parts). `ParseSkin(fsys)`
  layers a host's `*.tmpl` files over the built-in Bulma skin, so a host
  overrides only the templates it needs; set it as `Explorer.Skin`.
- Components. `Explorer.Catalog` (a `Catalog` interface; `StaticCatalog`
  for components fixed in code) divides the database into components that
  own tables and publish contract views. `/c/{component}` and
  `/c/{component}/{table}` scope the explorer to one component; foreign
  keys into another component link into its scope. The unscoped index
  lists the components and tags each table with its owner.
  `TableOptions.Component` scopes `TableHTMLWith`.
- The demo library is split into catalogue and lending components, with a
  `contract_books` view, and both skins browse it per component.
- The demo explores the same database through two skins: Bulma at `/` and a
  plain, classless skin at `/plain` that shows data rows as records. The
  WASM demo switches skin and stylesheet by URL.

### Changed
- `&` in emitted link URLs is now written `&amp;` (correct HTML; browsers and
  `getAttribute` decode it). Tests matching raw `href` strings need updating.

## [0.2.1] - 2026-09-29

 - Version update

## [0.2.0] - 2026-09-29

 - Remove Explorer.Postgres: both backends use the PostgreSQL catalogs (needs go-postgres v0.6.0)

### Removed
- `Explorer.Postgres`. Both backends now use the PostgreSQL catalogs
  (`pg_tables`, `pg_views`, `information_schema`, `pg_indexes`), which pglike
  provides from go-postgres v0.6.0. On pglike, the schema section now shows PG
  type names, the `<table>_pkey` index and view columns, the same as real
  PostgreSQL. Delete the field from `Explorer` literals.

## [0.1.2] - 2026-09-27

### Added
- `Views()` lists the database's views; the index shows them in their own
  box and they browse like tables.
- `Annotate` hook: an HTML fragment (a badge, say) shown beside each table
  or view name in the index, so a host can label ownership.

## [0.1.1] - 2026-09-26

 - Cleaning

## [0.1.0] - 2026-09-25

 - First release. Supersedes lofidb: roadmap and WASM soak test moved here; PostgreSQL-only catalog path over pgx and pglike; FK links, column filters, column formatting

 - Supersedes [lofidb](https://git.bytestone.uk/hum3/lofidb), now archived:
   its roadmap moved to ROADMAP.md and its WASM data-size soak test to
   `wasm_soak_test.go` (`task test:wasm:soak`, wasip1 via wazero), which
   now also checks the explorer renders the soaked table.
 - PostgreSQL only. The catalog is read through a single
   `information_schema`/`pg_indexes` query path that runs unchanged on
   real PostgreSQL and on go-postgres's pglike driver (SQLite file,
   in-memory, WASM), which installs views of the same shape. The
   `Explorer.Postgres` flag and the SQLite PRAGMA branch are gone; index
   columns and uniqueness are parsed from `indexdef` on both backends, and
   the index table no longer shows an Origin column.
 - Table names, column names, types, foreign-key targets and constraint
   rules are now HTML-escaped wherever they are rendered as text.
   Previously only defaults, index names and cell values were.
 - Demo: `-dsn` (or `DBEXPLORER_DSN`) accepts a SQLite file path, opened
   with pglike; `postgres://` DSNs still go to pgx. `DBEXPLORER_PG_DSN`
   remains honoured.
 - Data browser: foreign-key cells link to the referenced table filtered to
   that key. New `filter`/`value` query parameters (parameterised, column
   validated) and `TableHTMLWith(name, TableOptions)`; `TableHTML` keeps
   its signature as a wrapper.
 - Column formatting: `Explorer.UUIDLen` shortens UUIDs (full value in the
   tooltip), `Explorer.TimeFormat` formats `time.Time` cells, and
   `Explorer.Format` is a per-cell hook. NULL now renders as a grey `NULL`
   rather than `<nil>`.
 - Demo: the sample library now uses UUID primary keys (deterministic, so
   ids are stable across runs); `-uuid-len` and `-time-format` flags.
 - go-postgres v0.5.13: `DEFAULT gen_random_uuid()` works on pglike in the
   PostgreSQL form, so the demo DDL is identical for both backends.
 - Table pages: Schema is now a collapsible master section (closed by
   default) with Foreign Keys and Indexes nested inside it as their own
   collapsible detail sections; summaries show column/key/index counts.
   Uses native `<details>`, so no JavaScript is needed.
 - `Explorer.Footer`: optional HTML fragment rendered by `Handler` below
   the content.
 - Demos report their build version and last-commit timestamp (native:
   title, footer and startup log; WASM: subtitle via `goVersion()`),
   stamped by the Taskfile from git on every build.
 - Initial extraction of the DB explorer from gobank's Model Bank demo:
   `Explorer` type over `database/sql` with PostgreSQL and pglike/SQLite
   catalog dialects, `IndexHTML`/`TableHTML` fragments, path-routed
   `Render`, and a standalone-page `Handler`.
 - Demo (`cmd/demo`) with a sample lending-library database: native server
   against in-memory pglike or real PostgreSQL (`-dsn` /
   `DBEXPLORER_PG_DSN`), and a WASM build running the same database in the
   browser.
