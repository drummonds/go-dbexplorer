# go-dbexplorer

A lightweight HTML database explorer for Go's `database/sql`: table catalog
with row counts, per-table schema, foreign keys, indexes, and a paginated,
sortable, filterable data browser with foreign-key links and configurable
column formatting. Extracted from the
[gobank](https://git.bytestone.uk/hum3/gobank) Model Bank demo.

The explorer is PostgreSQL-specific: it reads `information_schema` and
`pg_indexes` and uses `$1` placeholders. One query path serves three
deployments:

- **PostgreSQL** proper, opened with `pgx`
- **SQLite file or in-memory**, opened with
  [go-postgres](https://git.bytestone.uk/hum3/go-postgres)'s `pglike`
  driver, which speaks PostgreSQL SQL over SQLite and installs the same
  catalog views on every connection
- **WASM**, the same pglike database running entirely in the browser

Pages render through a skin of `html/template`s. The built-in skin emits
[Bulma](https://bulma.io/) class names and renders unstyled without it; a
host can replace any of its templates (see [Skins](#skins)). Every
identifier and value is HTML-escaped.

## Library

```go
import dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"

ex := &dbexplorer.Explorer{
    DB:         db,                   // *sql.DB opened with pgx or pglike
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

### Components

A `Catalog` divides the database into components, each owning some tables
and publishing contract views for the others to read. With one set, the
explorer can be scoped to a single component:

```go
ex.Catalog = dbexplorer.StaticCatalog{
    {Name: "catalogue", Tables: []string{"authors", "books"}, Views: []string{"contract_books"}},
    {Name: "lending", Tables: []string{"members", "loans"}},
}
```

`/c/catalogue` lists only that component's tables and the views it
publishes, and `/c/catalogue/books` browses a table within the scope; a
table outside the component is not found there. Links stay in the scope,
except a foreign key into another component's table, which links into that
component's scope. The unscoped index lists the components and tags each
table with its owner. `Catalog` is an interface, so the components can
come from code or from the database itself. Without a catalog, `/c/…` has
no special meaning.

### Skins

A skin is three templates over exported view models: `index`
(`IndexView`), `table` (`TableView`) and `page` (`PageView`, the document
`Handler` wraps around a page). The view models carry display-ready data —
formatted cell text, every link already built — so a template only arranges
it. `ParseSkin` layers a host's `*.tmpl` files over the built-in Bulma skin,
so a host overrides just the templates it needs, and reports template
errors at startup:

```go
//go:embed skins/mine/*.tmpl
var mine embed.FS

sub, _ := fs.Sub(mine, "skins/mine")
skin, err := dbexplorer.ParseSkin(sub) // defines "index", "table" and/or "page"
ex.Skin = skin                         // nil = built-in Bulma
```

Templates may restructure the page, not just restyle it: the demo's
[plain skin](cmd/demo/skins/plain) renders classless HTML for simple.css
and shows each data row as a record rather than a table row. `Annotate` and
`Footer` are the only trusted HTML; everything else is escaped.

## Demo

The demo explores a sample lending-library database (authors, books,
members, loans — UUID keys, foreign keys and indexes included), on any of
the backends, divided into two components (catalogue and lending), through
two skins: the built-in Bulma one at `/` and the
demo's plain skin at `/plain`:

```sh
task demo                                   # pglike, in-memory: http://localhost:8080/
task demo -- -dsn /tmp/library.db           # pglike on a SQLite file (seeds if absent)
task demo -- -dsn postgres://user:pw@host/db  # real PostgreSQL (seeds if absent)
DBEXPLORER_DSN=postgres://... task demo       # same, via env
task demo -- -uuid-len 0 -time-format "02 Jan 2006"  # column formatting
```

A `postgres://` or `postgresql://` DSN opens PostgreSQL through pgx; any
other non-empty DSN is handed to pglike as a SQLite file path.

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
assertions against real PostgreSQL as well, for example with a throwaway
container:

```sh
podman run -d --rm --name pg -e POSTGRES_HOST_AUTH_METHOD=trust \
  -e POSTGRES_DB=dbexplorer_test -p 127.0.0.1:55432:5432 docker.io/library/postgres:16-alpine
DBEXPLORER_PG_DSN="postgres://postgres@127.0.0.1:55432/dbexplorer_test?sslmode=disable" task test
podman stop pg
```

The WASM soak test (`task test:wasm:soak`, moved from lofidb) fills a
pglike `:memory:` database under wasip1 via wazero, records heap use as
JSON lines (`WASM_SOAK_ROWS`, `WASM_SOAK_OUTPUT`) and checks the explorer
still renders the table. See ROADMAP.md for what is planned.
