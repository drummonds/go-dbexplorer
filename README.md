# go-dbexplorer

A lightweight HTML database explorer for Go's `database/sql`: table catalog
with row counts, per-table schema, foreign keys, indexes, and a paginated,
sortable data browser with optional value truncation. Extracted from the
[gobank](https://git.bytestone.uk/hum3/gobank) Model Bank demo.

Two catalog dialects are supported:

- **PostgreSQL** — `pg_tables`, `information_schema`, `pg_indexes`
- **SQLite-style** — `sqlite_master` + `PRAGMA`, as exposed by
  [go-postgres](https://git.bytestone.uk/hum3/go-postgres)'s `pglike`
  driver (PostgreSQL SQL on SQLite, including in WASM)

The emitted HTML uses [Bulma](https://bulma.io/) class names; pages render
unstyled without it.

## Library

```go
import dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"

ex := &dbexplorer.Explorer{
    DB:       db,                  // *sql.DB
    Postgres: true,                // false = pglike/SQLite catalogs
    BasePath: "/internal/explorer" // prefix for emitted links
}

html := ex.IndexHTML()                                  // overview fragment
html  = ex.TableHTML("books", 1, "title", "asc", false) // table fragment
html  = ex.Render("/internal/explorer/books?page=2")    // path-routed
http.Handle("/", ex.Handler())                          // standalone pages
```

`IndexHTML`/`TableHTML` return HTML fragments for embedding in your own
layout; `Handler` serves complete standalone pages (Bulma via CDN).
`Render` is the routing-free core, useful where navigation is dispatched by
the host — the WASM demo calls it from JavaScript.

## Demo

The demo explores a sample lending-library database (authors, books,
members, loans — foreign keys and indexes included), in either backend:

```sh
task demo                                   # pglike, in-memory: http://localhost:8080/
task demo -- -dsn postgres://user:pw@host/db  # real PostgreSQL (seeds if absent)
DBEXPLORER_PG_DSN=postgres://... task demo    # same, via env
```

The WASM version runs the identical database and explorer entirely in the
browser (`task docs:build`, then serve `docs/`); it is published at
<https://go-dbexplorer.docs.bytestone.uk/>.

## Tests

`task test` runs against pglike; set `DBEXPLORER_PG_DSN` to run the same
assertions against real PostgreSQL as well.
