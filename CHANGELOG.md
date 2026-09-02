# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

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
