# go-dbexplorer

A lightweight HTML database explorer for Go's `database/sql`: table catalog
with row counts, per-table schema, foreign keys, indexes, and a paginated,
sortable, filterable data browser with foreign-key links and configurable
column formatting. Extracted from the
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
    DB:         db,                   // *sql.DB
    Postgres:   true,                 // false = pglike/SQLite catalogs
    BasePath:   "/internal/explorer", // prefix for emitted links
    UUIDLen:    8,                    // show UUIDs as 8 chars + …, full in tooltip
    TimeFormat: "2006-01-02 15:04",   // Go layout for time.Time cells
}

html := ex.IndexHTML()                                  // overview fragment
html  = ex.TableHTML("books", 1, "title", "asc", false) // table fragment
html  = ex.TableHTMLWith("books", dbexplorer.TableOptions{
    Sort: "title", Dir: "asc", FilterCol: "author_id", FilterVal: id})
html  = ex.Render("/internal/explorer/books?page=2")    // path-routed
http.Handle("/", ex.Handler())                          // standalone pages
```

`IndexHTML`/`TableHTML`/`TableHTMLWith` return HTML fragments for embedding
in your own layout; `Handler` serves complete standalone pages (Bulma via
CDN), with optional `Title` and `Footer` (an HTML fragment shown below the
content). `Render` is the routing-free core, useful where navigation is
dispatched by the host — the WASM demo calls it from JavaScript. Its query
string carries `page`, `sort`, `dir`, `trunc`, and `filter`/`value`.

### Data browser

- **Foreign keys are links.** A cell in a foreign-key column links to the
  referenced table filtered to that key (`/authors?filter=id&value=…`).
  The filter is applied as a query parameter, never spliced into SQL, and
  the filtered page shows the filter with a link to clear it.
- **Column formatting.** `UUIDLen` shortens values in canonical UUID form
  (any column type — pglike stores UUIDs as TEXT) with the full value in
  the hover tooltip; `TimeFormat` is the Go layout for `time.Time` values;
  NULL renders as a grey `NULL`. `Format` is an escape hatch called first
  for every non-NULL cell:

  ```go
  ex.Format = func(table, column string, v any) (string, bool) {
      if column == "amount_pence" {
          return fmt.Sprintf("£%.2f", float64(v.(int64))/100), true
      }
      return "", false // fall through to the built-in formatting
  }
  ```
- **Truncate toggle.** The per-page `trunc=1` option additionally cuts any
  text cell to 10 characters.

## Demo

The demo explores a sample lending-library database (authors, books,
members, loans — UUID keys, foreign keys and indexes included), in either
backend:

```sh
task demo                                   # pglike, in-memory: http://localhost:8080/
task demo -- -dsn postgres://user:pw@host/db  # real PostgreSQL (seeds if absent)
DBEXPLORER_PG_DSN=postgres://... task demo    # same, via env
task demo -- -uuid-len 0 -time-format "02 Jan 2006"  # column formatting
```

The WASM version runs the identical database and explorer entirely in the
browser (`task docs:build`, then serve `docs/`); it is published at
<https://go-dbexplorer.docs.bytestone.uk/>.

Both demos show their build version and the timestamp of the last commit
(page footer / subtitle, and the server's startup log). The Taskfile stamps
these from `git describe --tags --always --dirty` and the HEAD commit time
on every build, so each commit is reflected automatically; pass
`VERSION=vX.Y.Z` to override the tag (as `tp release` does). A plain
`go build` falls back to the VCS revision Go embeds itself.

## Tests

`task test` runs against pglike; set `DBEXPLORER_PG_DSN` to run the same
assertions against real PostgreSQL as well.
