# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

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
