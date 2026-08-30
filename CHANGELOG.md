# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

 - Initial extraction of the DB explorer from gobank's Model Bank demo:
   `Explorer` type over `database/sql` with PostgreSQL and pglike/SQLite
   catalog dialects, `IndexHTML`/`TableHTML` fragments, path-routed
   `Render`, and a standalone-page `Handler`.
 - Demo (`cmd/demo`) with a sample lending-library database: native server
   against in-memory pglike or real PostgreSQL (`-dsn` /
   `DBEXPLORER_PG_DSN`), and a WASM build running the same database in the
   browser.
