# Roadmap

Carried over from lofidb (archived
2026-09-25, deleted 2026-09-28), which this library superseded. Items lofidb planned that were
already delivered here are ticked with the version.

## Next

- [ ] Standalone CLI: `dbexplorer postgres://…` or `dbexplorer file.db`, auto-selecting pgx or pglike (lofidb had one)
- [ ] `HideTables` and `Schema` options on `Explorer` (lofidb had both; the schema is hard-coded to `public`)
- [ ] Detect the schema(s) the user can read instead of hard-coding `public`
- [ ] Search/filter on the table list
- [ ] Export row data as CSV / JSON
- [ ] Read-only DDL preview (formatted CREATE TABLE)
- [ ] Dark mode toggle (CSS class swap)

## Editing (opt-in)

- [ ] Inline cell edit behind an `AllowWrite` gate
- [ ] INSERT / DELETE row from the table view
- [ ] Audit log of write actions

## Schema visualisation

- [ ] FK relationship diagram (d2-rendered or pure SVG)

## Done

- [x] Column-level filters on the table view (`filter`/`value`) — v0.1.0
- [x] Click-through between related tables via foreign-key links — v0.1.0
- [x] Index columns on real PostgreSQL without helper SQL (parsed from `indexdef`) — v0.1.0
- [x] WASM data-size soak test, moved from lofidb (`task test:wasm:soak`) — v0.1.0

## Maybe / not committed

- HTMX-based incremental updates rather than full-page reloads
- `EXPLAIN` viewer for arbitrary user queries
- Materialised view discovery
- Multi-schema browsing in one session
