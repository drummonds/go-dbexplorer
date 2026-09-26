// Package dbexplorer renders a lightweight HTML database explorer — table
// catalog, per-table schema, foreign keys, indexes, and a paginated,
// sortable data browser — for any database opened through database/sql.
//
// Two catalog dialects are understood: real PostgreSQL
// (pg_tables/information_schema/pg_indexes) and SQLite-style engines such
// as pglike (sqlite_master/PRAGMA), selected by Explorer.Postgres. The
// emitted HTML uses Bulma CSS class names and is unstyled without it.
package dbexplorer

import (
	"database/sql"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Explorer renders explorer pages for one database.
type Explorer struct {
	DB *sql.DB
	// Postgres selects the real PostgreSQL system catalogs; false uses the
	// SQLite-style catalog (sqlite_master/PRAGMA) of engines like pglike.
	Postgres bool
	// BasePath prefixes every link the explorer emits, e.g.
	// "/internal/explorer". Empty means links from the root ("/books").
	BasePath string
	// Title is the page <title> used by Handler. Default "DB Explorer".
	Title string
	// Footer is an optional HTML fragment Handler renders below the content
	// (a version line, say). It is emitted as-is, not escaped.
	Footer string

	// UUIDLen shortens UUID values in the data browser to this many
	// characters, keeping the full value in the hover tooltip. 0 shows them
	// in full. UUIDs are recognised by their canonical 8-4-4-4-12 form.
	UUIDLen int
	// TimeFormat is the Go time layout used for time.Time values in the data
	// browser, e.g. "2006-01-02 15:04". Empty uses Go's default formatting.
	TimeFormat string
	// Format, when set, is consulted first for every non-NULL data cell.
	// Return the display text and true to use it; false falls through to the
	// built-in UUID/time/truncation formatting.
	Format func(table, column string, v any) (string, bool)
	// Annotate, when set, returns an HTML fragment (a badge, say) shown
	// beside each table or view name in the index. It is emitted as-is.
	Annotate func(name string) string
}

// TableOptions selects what TableHTMLWith renders for a table.
type TableOptions struct {
	Page      int    // 1-based page of 50 rows; values below 1 mean the first page
	Sort, Dir string // ORDER BY column (ignored unless it exists) and "asc"/"desc"
	Trunc     bool   // shorten text cells to 10 characters, full value in the tooltip
	// FilterCol/FilterVal restrict the rows to those where FilterCol equals
	// FilterVal (compared as a query parameter). FilterCol must be a column
	// of the table, otherwise no filter is applied. Foreign-key cells link to
	// the referenced table with this filter set to the key.
	FilterCol, FilterVal string
}

func (e *Explorer) tableURL(name string) string {
	return e.BasePath + "/" + url.PathEscape(name)
}

func (e *Explorer) indexURL() string {
	if e.BasePath == "" {
		return "/"
	}
	return e.BasePath
}

// Render renders the page for a URL relative to BasePath: the index for ""
// or "/", otherwise the table page for "/<table>". The query string
// supplies page, sort, dir, trunc, filter and value (see TableOptions). This
// is the routing-free core used by Handler and by WASM hosts that dispatch
// navigation themselves.
func (e *Explorer) Render(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return e.IndexHTML()
	}
	name := strings.Trim(strings.TrimPrefix(u.Path, e.BasePath), "/")
	if name != "" {
		if unescaped, err := url.PathUnescape(name); err == nil {
			name = unescaped
		}
	}
	if name == "" {
		return e.IndexHTML()
	}
	q := u.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	return e.TableHTMLWith(name, TableOptions{
		Page: page, Sort: q.Get("sort"), Dir: q.Get("dir"), Trunc: q.Get("trunc") == "1",
		FilterCol: q.Get("filter"), FilterVal: q.Get("value"),
	})
}

// Handler serves the explorer as standalone HTML pages (Bulma via CDN).
// Mount it at BasePath (or at "/" when BasePath is empty).
func (e *Explorer) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		title := e.Title
		if title == "" {
			title = "DB Explorer"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">
</head><body><section class="section"><div class="container">`, html.EscapeString(title))
		fmt.Fprint(w, e.Render(r.URL.String()))
		if e.Footer != "" {
			fmt.Fprintf(w, `<footer class="mt-5 is-size-7 has-text-grey">%s</footer>`, e.Footer)
		}
		fmt.Fprint(w, `</div></section></body></html>`)
	})
}

// The explorer reads catalog metadata through the helpers below, which
// branch on the backend: pglike exposes SQLite-style sqlite_master/PRAGMA,
// real PostgreSQL uses information_schema/pg_catalog.

// dbColInfo describes one column of a table.
type dbColInfo struct {
	pos     int
	name    string
	ctype   string
	notNull bool
	dflt    string
	pk      bool
}

// dbFKInfo describes one foreign key reference.
type dbFKInfo struct {
	fromCol, refTable, toCol, onUpdate, onDelete string
}

// dbIdxInfo describes one index.
type dbIdxInfo struct {
	name    string
	columns string
	unique  bool
	origin  string
}

// Tables returns all user table names.
func (e *Explorer) Tables() []string {
	if e.DB == nil {
		return nil
	}
	q := `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`
	if e.Postgres {
		q = `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`
	}
	rows, err := e.DB.Query(q)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			tables = append(tables, name)
		}
	}
	return tables
}

// Views returns all user view names.
func (e *Explorer) Views() []string {
	if e.DB == nil {
		return nil
	}
	q := `SELECT name FROM sqlite_master WHERE type='view' ORDER BY name`
	if e.Postgres {
		q = `SELECT viewname FROM pg_views WHERE schemaname = 'public' ORDER BY viewname`
	}
	rows, err := e.DB.Query(q)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var views []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			views = append(views, name)
		}
	}
	return views
}

// annotation is the Annotate fragment for a name, with a leading space, or
// nothing when no hook is set.
func (e *Explorer) annotation(name string) string {
	if e.Annotate == nil {
		return ""
	}
	if a := e.Annotate(name); a != "" {
		return " " + a
	}
	return ""
}

// tableColumns returns the columns of a table in definition order.
func (e *Explorer) tableColumns(name string) []dbColInfo {
	if e.DB == nil {
		return nil
	}
	var cols []dbColInfo
	if e.Postgres {
		rows, err := e.DB.Query(`SELECT c.ordinal_position, c.column_name, c.data_type,
				c.is_nullable = 'NO', COALESCE(c.column_default, ''), COALESCE(pk.is_pk, false)
			FROM information_schema.columns c
			LEFT JOIN (SELECT kcu.column_name, true AS is_pk
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
					AND tc.table_schema = 'public' AND tc.table_name = $1
			) pk ON pk.column_name = c.column_name
			WHERE c.table_schema = 'public' AND c.table_name = $1
			ORDER BY c.ordinal_position`, name)
		if err != nil {
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var c dbColInfo
			if rows.Scan(&c.pos, &c.name, &c.ctype, &c.notNull, &c.dflt, &c.pk) == nil {
				cols = append(cols, c)
			}
		}
		return cols
	}
	rows, err := e.DB.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, name))
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var c dbColInfo
		var notnull, pk int
		var dflt *string
		if rows.Scan(&c.pos, &c.name, &c.ctype, &notnull, &dflt, &pk) == nil {
			c.notNull = notnull == 1
			c.pk = pk > 0
			if dflt != nil {
				c.dflt = *dflt
			}
			cols = append(cols, c)
		}
	}
	return cols
}

// tableFKs returns the foreign keys declared on a table.
func (e *Explorer) tableFKs(name string) []dbFKInfo {
	if e.DB == nil {
		return nil
	}
	var fks []dbFKInfo
	if e.Postgres {
		rows, err := e.DB.Query(`SELECT kcu.column_name, ccu.table_name, ccu.column_name,
				rc.update_rule, rc.delete_rule
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema
			JOIN information_schema.constraint_column_usage ccu
				ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
			JOIN information_schema.referential_constraints rc
				ON rc.constraint_name = tc.constraint_name AND rc.constraint_schema = tc.table_schema
			WHERE tc.constraint_type = 'FOREIGN KEY'
				AND tc.table_schema = 'public' AND tc.table_name = $1`, name)
		if err != nil {
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var f dbFKInfo
			if rows.Scan(&f.fromCol, &f.refTable, &f.toCol, &f.onUpdate, &f.onDelete) == nil {
				fks = append(fks, f)
			}
		}
		return fks
	}
	rows, err := e.DB.Query(fmt.Sprintf(`PRAGMA foreign_key_list("%s")`, name))
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var id, seq int
		var f dbFKInfo
		var match string
		if rows.Scan(&id, &seq, &f.refTable, &f.fromCol, &f.toCol, &f.onUpdate, &f.onDelete, &match) == nil {
			fks = append(fks, f)
		}
	}
	return fks
}

// tableIndexes returns the indexes on a table.
func (e *Explorer) tableIndexes(name string) []dbIdxInfo {
	if e.DB == nil {
		return nil
	}
	var idxs []dbIdxInfo
	if e.Postgres {
		rows, err := e.DB.Query(`SELECT indexname, indexdef FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = $1 ORDER BY indexname`, name)
		if err != nil {
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var ix dbIdxInfo
			var def string
			if rows.Scan(&ix.name, &def) != nil {
				continue
			}
			ix.unique = strings.HasPrefix(def, "CREATE UNIQUE")
			if lp, rp := strings.Index(def, "("), strings.LastIndex(def, ")"); lp >= 0 && rp > lp {
				ix.columns = def[lp+1 : rp]
			}
			if strings.HasSuffix(ix.name, "_pkey") {
				ix.origin = "pk"
			}
			idxs = append(idxs, ix)
		}
		return idxs
	}
	rows, err := e.DB.Query(fmt.Sprintf(`PRAGMA index_list("%s")`, name))
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var unique, partial int
		var ix dbIdxInfo
		if rows.Scan(&seq, &ix.name, &unique, &ix.origin, &partial) != nil {
			continue
		}
		ix.unique = unique == 1
		var cols []string
		ixColRows, err := e.DB.Query(fmt.Sprintf(`PRAGMA index_info("%s")`, ix.name))
		if err == nil {
			for ixColRows.Next() {
				var seqno, cid int
				var colName string
				if ixColRows.Scan(&seqno, &cid, &colName) == nil {
					cols = append(cols, colName)
				}
			}
			ixColRows.Close()
		}
		ix.columns = strings.Join(cols, ", ")
		idxs = append(idxs, ix)
	}
	return idxs
}

// IndexHTML renders the explorer overview: all tables with row/column
// counts and foreign-key relationships.
func (e *Explorer) IndexHTML() string {
	var s strings.Builder
	s.WriteString(`<h2 class="title is-4">DB Explorer</h2>`)

	if e.DB == nil {
		s.WriteString(`<p class="has-text-danger">Database not available.</p>`)
		return s.String()
	}

	tables := e.Tables()
	if len(tables) == 0 {
		s.WriteString(`<p class="has-text-grey">No tables found.</p>`)
		return s.String()
	}

	// Table list with row/column counts
	s.WriteString(`<div class="box">`)
	s.WriteString(`<h3 class="title is-5">Tables</h3>`)
	s.WriteString(`<table class="table is-fullwidth is-striped">`)
	s.WriteString(`<thead><tr><th>Table</th><th class="has-text-right">Columns</th><th class="has-text-right">Rows</th></tr></thead>`)
	s.WriteString(`<tbody>`)

	for _, name := range tables {
		var rowCount int
		err := e.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, name)).Scan(&rowCount)
		rowStr := fmt.Sprintf("%d", rowCount)
		if err != nil {
			rowStr = "error"
		}
		colCount := len(e.tableColumns(name))
		s.WriteString(fmt.Sprintf(`<tr><td><a href="%s">%s</a>%s</td><td class="has-text-right">%d</td><td class="has-text-right">%s</td></tr>`,
			e.tableURL(name), name, e.annotation(name), colCount, rowStr))
	}

	s.WriteString(`</tbody></table>`)
	s.WriteString(`</div>`)

	if views := e.Views(); len(views) > 0 {
		s.WriteString(`<div class="box">`)
		s.WriteString(`<h3 class="title is-5">Views</h3>`)
		s.WriteString(`<table class="table is-fullwidth is-striped">`)
		s.WriteString(`<thead><tr><th>View</th><th class="has-text-right">Columns</th><th class="has-text-right">Rows</th></tr></thead>`)
		s.WriteString(`<tbody>`)
		for _, name := range views {
			var rowCount int
			err := e.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, name)).Scan(&rowCount)
			rowStr := fmt.Sprintf("%d", rowCount)
			if err != nil {
				rowStr = "error"
			}
			s.WriteString(fmt.Sprintf(`<tr><td><a href="%s">%s</a>%s</td><td class="has-text-right">%d</td><td class="has-text-right">%s</td></tr>`,
				e.tableURL(name), name, e.annotation(name), len(e.tableColumns(name)), rowStr))
		}
		s.WriteString(`</tbody></table>`)
		s.WriteString(`</div>`)
	}

	// Foreign key relationships
	type fkRef struct {
		from, fromCol, to, toCol string
	}
	var refs []fkRef

	for _, name := range tables {
		for _, f := range e.tableFKs(name) {
			refs = append(refs, fkRef{from: name, fromCol: f.fromCol, to: f.refTable, toCol: f.toCol})
		}
	}

	if len(refs) > 0 {
		s.WriteString(`<div class="box">`)
		s.WriteString(`<h3 class="title is-5">Relationships</h3>`)
		s.WriteString(`<table class="table is-fullwidth is-striped is-narrow">`)
		s.WriteString(`<thead><tr><th>Table</th><th>Column</th><th></th><th>References</th><th>Column</th></tr></thead>`)
		s.WriteString(`<tbody>`)
		for _, r := range refs {
			s.WriteString(fmt.Sprintf(`<tr><td><a href="%s">%s</a></td><td>%s</td><td>&rarr;</td><td><a href="%s">%s</a></td><td>%s</td></tr>`,
				e.tableURL(r.from), r.from, r.fromCol, e.tableURL(r.to), r.to, r.toCol))
		}
		s.WriteString(`</tbody></table>`)
		s.WriteString(`</div>`)
	}

	return s.String()
}

// truncLen is the display length text cell values are cut to when the
// explorer's truncate option is on; the full value stays in the hover tooltip.
const truncLen = 10

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// rawText is the plain text of a value, as used for filter links: the
// canonical form for UUID bytes, the string for text, fmt's default otherwise.
func rawText(v any) string {
	switch x := v.(type) {
	case [16]byte:
		return fmt.Sprintf("%x-%x-%x-%x-%x", x[0:4], x[4:6], x[6:8], x[8:10], x[10:16])
	case []byte:
		return string(x)
	case string:
		return x
	case time.Time:
		return x.Format(time.RFC3339Nano)
	}
	return fmt.Sprint(v)
}

// formatValue returns the full display text of a value and, when it should
// be shown shortened, the prefix to display instead (the full text then goes
// in the tooltip). Times honour TimeFormat; UUIDs honour UUIDLen; other text
// honours the trunc toggle. Numbers are never cut.
func (e *Explorer) formatValue(v any, trunc bool) (full, short string) {
	switch x := v.(type) {
	case time.Time:
		if e.TimeFormat != "" {
			return x.Format(e.TimeFormat), ""
		}
		return fmt.Sprint(x), ""
	case string, []byte, [16]byte:
		full = rawText(x)
	default:
		return fmt.Sprint(x), ""
	}
	if uuidRe.MatchString(full) {
		if e.UUIDLen > 0 && e.UUIDLen < len(full) {
			return full, full[:e.UUIDLen]
		}
		return full, ""
	}
	if r := []rune(full); trunc && len(r) > truncLen {
		return full, string(r[:truncLen])
	}
	return full, ""
}

// cell renders one data cell: NULL in grey, otherwise the value formatted
// by Format (if set) or formatValue, wrapped in a link when href is given.
func (e *Explorer) cell(table, column string, v any, trunc bool, href string) string {
	if v == nil {
		return `<td class="has-text-grey-light">NULL</td>`
	}
	var full, short string
	handled := false
	if e.Format != nil {
		full, handled = e.Format(table, column, v)
	}
	if !handled {
		full, short = e.formatValue(v, trunc)
	}
	text, title := html.EscapeString(full), ""
	if short != "" {
		text = html.EscapeString(short) + "&hellip;"
		title = fmt.Sprintf(` title="%s"`, html.EscapeString(full))
	}
	if href != "" {
		text = fmt.Sprintf(`<a href="%s">%s</a>`, href, text)
	}
	return fmt.Sprintf(`<td%s>%s</td>`, title, text)
}

// tableLink builds a link to a table page carrying the given options.
func (e *Explorer) tableLink(name string, o TableOptions) string {
	q := url.Values{}
	if o.Page > 1 {
		q.Set("page", strconv.Itoa(o.Page))
	}
	if o.Sort != "" {
		q.Set("sort", o.Sort)
		q.Set("dir", o.Dir)
	}
	if o.Trunc {
		q.Set("trunc", "1")
	}
	if o.FilterCol != "" {
		q.Set("filter", o.FilterCol)
		q.Set("value", o.FilterVal)
	}
	if len(q) == 0 {
		return e.tableURL(name)
	}
	return e.tableURL(name) + "?" + q.Encode()
}

// TableHTML renders the detail view for a single table: schema, foreign
// keys, indexes, and paginated data. It is TableHTMLWith without a filter;
// trunc shortens text cells to truncLen characters.
func (e *Explorer) TableHTML(name string, page int, sort string, dir string, trunc bool) string {
	return e.TableHTMLWith(name, TableOptions{Page: page, Sort: sort, Dir: dir, Trunc: trunc})
}

// TableHTMLWith renders the detail view for a single table — schema with
// foreign keys and indexes, then the paginated, sortable, optionally
// filtered data — as an HTML fragment. Sort, pagination and filter state is
// carried through every emitted link. Cells in foreign-key columns link to
// the referenced table filtered to that key.
func (e *Explorer) TableHTMLWith(name string, o TableOptions) string {
	var s strings.Builder

	if e.DB == nil {
		s.WriteString(`<p class="has-text-danger">Database not available.</p>`)
		return s.String()
	}

	// Validate table name against actual DB catalog
	tables := e.Tables()
	valid := slices.Contains(tables, name) || slices.Contains(e.Views(), name)
	if !valid {
		s.WriteString(`<h2 class="title is-4">Table Not Found</h2>`)
		s.WriteString(fmt.Sprintf(`<p class="has-text-danger">Unknown table: %s</p>`, html.EscapeString(name)))
		s.WriteString(fmt.Sprintf(`<p><a href="%s">&larr; Back to explorer</a></p>`, e.indexURL()))
		return s.String()
	}

	s.WriteString(fmt.Sprintf(`<h2 class="title is-4">Table: %s</h2>`, name))
	s.WriteString(fmt.Sprintf(`<p class="mb-4"><a href="%s">&larr; Back to explorer</a></p>`, e.indexURL()))

	// --- Schema (master, closed by default) with Foreign Keys and Indexes
	// as nested detail sections. Native <details> keeps this JS-free so it
	// behaves the same in Handler pages and the WASM demo.
	var colNames []string
	schemaCols := e.tableColumns(name)
	fks := e.tableFKs(name)
	idxs := e.tableIndexes(name)

	s.WriteString(`<details class="box">`)
	s.WriteString(fmt.Sprintf(`<summary class="title is-5" style="cursor:pointer">Schema (%d columns)</summary>`, len(schemaCols)))
	if len(schemaCols) > 0 {
		s.WriteString(`<table class="table is-fullwidth is-striped is-narrow">`)
		s.WriteString(`<thead><tr><th>#</th><th>Name</th><th>Type</th><th>Not Null</th><th>Default</th><th>PK</th></tr></thead>`)
		s.WriteString(`<tbody>`)
		for _, c := range schemaCols {
			colNames = append(colNames, c.name)
			nnStr := ""
			if c.notNull {
				nnStr = "YES"
			}
			pkStr := ""
			if c.pk {
				pkStr = "YES"
			}
			s.WriteString(fmt.Sprintf(`<tr><td>%d</td><td><strong>%s</strong></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				c.pos, c.name, c.ctype, nnStr, html.EscapeString(c.dflt), pkStr))
		}
		s.WriteString(`</tbody></table>`)
	} else {
		s.WriteString(`<p class="has-text-danger">Schema not available.</p>`)
	}

	// --- Foreign Keys (nested detail) ---
	if len(fks) > 0 {
		s.WriteString(`<details class="mt-4">`)
		s.WriteString(fmt.Sprintf(`<summary class="title is-6" style="cursor:pointer">Foreign Keys (%d)</summary>`, len(fks)))
		s.WriteString(`<table class="table is-fullwidth is-striped is-narrow">`)
		s.WriteString(`<thead><tr><th>Column</th><th>References</th><th>Column</th><th>On Update</th><th>On Delete</th></tr></thead>`)
		s.WriteString(`<tbody>`)
		for _, f := range fks {
			s.WriteString(fmt.Sprintf(`<tr><td>%s</td><td><a href="%s">%s</a></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				f.fromCol, e.tableURL(f.refTable), f.refTable, f.toCol, f.onUpdate, f.onDelete))
		}
		s.WriteString(`</tbody></table>`)
		s.WriteString(`</details>`)
	}

	// --- Indexes (nested detail) ---
	if len(idxs) > 0 {
		s.WriteString(`<details class="mt-4">`)
		s.WriteString(fmt.Sprintf(`<summary class="title is-6" style="cursor:pointer">Indexes (%d)</summary>`, len(idxs)))
		s.WriteString(`<table class="table is-fullwidth is-striped is-narrow">`)
		s.WriteString(`<thead><tr><th>Name</th><th>Columns</th><th>Unique</th><th>Origin</th></tr></thead>`)
		s.WriteString(`<tbody>`)
		for _, ix := range idxs {
			uniq := ""
			if ix.unique {
				uniq = "YES"
			}
			s.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				html.EscapeString(ix.name), html.EscapeString(ix.columns), uniq, ix.origin))
		}
		s.WriteString(`</tbody></table>`)
		s.WriteString(`</details>`)
	}
	s.WriteString(`</details>`)

	// --- Data browser ---
	const pageSize = 50
	sort, dir := o.Sort, o.Dir
	if !slices.Contains(colNames, sort) {
		sort = ""
	}
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}
	filterCol, filterVal := o.FilterCol, o.FilterVal
	if !slices.Contains(colNames, filterCol) {
		filterCol, filterVal = "", ""
	}
	where := ""
	var args []any
	if filterCol != "" {
		where = fmt.Sprintf(` WHERE "%s" = $1`, filterCol)
		args = append(args, filterVal)
	}

	var totalRows int
	e.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"%s`, name, where), args...).Scan(&totalRows)
	totalPages := max((totalRows+pageSize-1)/pageSize, 1)
	page := min(max(o.Page, 1), totalPages)
	offset := (page - 1) * pageSize

	// cur is the effective state every link on this page is derived from.
	cur := TableOptions{Page: page, Sort: sort, Dir: dir, Trunc: o.Trunc, FilterCol: filterCol, FilterVal: filterVal}

	// Foreign-key columns link through to the referenced table.
	fkByCol := map[string]dbFKInfo{}
	for _, f := range fks {
		fkByCol[f.fromCol] = f
	}

	s.WriteString(`<div class="box">`)
	s.WriteString(fmt.Sprintf(`<h3 class="title is-5">Data (%d rows)</h3>`, totalRows))

	if filterCol != "" {
		clear := cur
		clear.Page, clear.FilterCol, clear.FilterVal = 1, "", ""
		s.WriteString(fmt.Sprintf(`<p class="mb-3"><span class="tag is-info is-light">Filter: %s = %s</span> <a class="is-size-7" href="%s">clear filter</a></p>`,
			html.EscapeString(filterCol), html.EscapeString(filterVal), e.tableLink(name, clear)))
	}

	// Truncation toggle, preserving page, sort and filter in the link.
	toggle := cur
	toggle.Trunc = !o.Trunc
	toggleLabel := "Truncate text to 10 chars"
	if o.Trunc {
		toggleLabel = "Show full values"
	}
	s.WriteString(fmt.Sprintf(`<p class="mb-3"><a class="button is-small" href="%s">%s</a></p>`,
		e.tableLink(name, toggle), toggleLabel))

	query := fmt.Sprintf(`SELECT * FROM "%s"%s`, name, where)
	if sort != "" {
		query += fmt.Sprintf(` ORDER BY "%s" %s`, sort, dir)
	}
	query += fmt.Sprintf(` LIMIT %d OFFSET %d`, pageSize, offset)

	dataRows, err := e.DB.Query(query, args...)
	if err != nil {
		s.WriteString(fmt.Sprintf(`<p class="has-text-danger">Query error: %v</p>`, err))
		s.WriteString(`</div>`)
		return s.String()
	}
	defer dataRows.Close()

	cols, _ := dataRows.Columns()
	if len(cols) > 0 {
		s.WriteString(`<div style="overflow-x:auto">`)
		s.WriteString(`<table class="table is-fullwidth is-striped is-narrow">`)
		s.WriteString(`<thead><tr>`)
		for _, col := range cols {
			hdr := cur
			hdr.Sort, hdr.Dir = col, "asc"
			arrow := ""
			if col == sort {
				if dir == "asc" {
					hdr.Dir = "desc"
					arrow = " &uarr;"
				} else {
					arrow = " &darr;"
				}
			}
			s.WriteString(fmt.Sprintf(`<th><a href="%s">%s%s</a></th>`, e.tableLink(name, hdr), col, arrow))
		}
		s.WriteString(`</tr></thead><tbody>`)

		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}

		rowCount := 0
		for dataRows.Next() {
			if dataRows.Scan(ptrs...) == nil {
				s.WriteString(`<tr>`)
				for i, v := range vals {
					href := ""
					if f, ok := fkByCol[cols[i]]; ok && v != nil {
						href = e.tableLink(f.refTable, TableOptions{Trunc: o.Trunc, FilterCol: f.toCol, FilterVal: rawText(v)})
					}
					s.WriteString(e.cell(name, cols[i], v, o.Trunc, href))
				}
				s.WriteString(`</tr>`)
				rowCount++
			}
		}
		s.WriteString(`</tbody></table></div>`)
		if rowCount == 0 {
			s.WriteString(`<p class="has-text-grey">No rows.</p>`)
		}
	}

	// Pagination — every link carries sort, trunc and filter.
	if totalPages > 1 {
		pageLink := func(p int) string {
			pp := cur
			pp.Page = p
			return e.tableLink(name, pp)
		}
		s.WriteString(`<nav class="pagination is-small mt-4" role="navigation">`)
		if page > 1 {
			s.WriteString(fmt.Sprintf(`<a class="pagination-previous" href="%s">Previous</a>`, pageLink(page-1)))
		} else {
			s.WriteString(`<a class="pagination-previous" disabled>Previous</a>`)
		}
		if page < totalPages {
			s.WriteString(fmt.Sprintf(`<a class="pagination-next" href="%s">Next</a>`, pageLink(page+1)))
		} else {
			s.WriteString(`<a class="pagination-next" disabled>Next</a>`)
		}
		s.WriteString(`<ul class="pagination-list">`)

		// Show page numbers: first, current-1, current, current+1, last
		shown := map[int]bool{}
		pagesToShow := []int{1}
		if page > 2 {
			pagesToShow = append(pagesToShow, page-1)
		}
		pagesToShow = append(pagesToShow, page)
		if page < totalPages-1 {
			pagesToShow = append(pagesToShow, page+1)
		}
		pagesToShow = append(pagesToShow, totalPages)

		lastShown := 0
		for _, p := range pagesToShow {
			if shown[p] {
				continue
			}
			shown[p] = true
			if lastShown > 0 && p > lastShown+1 {
				s.WriteString(`<li><span class="pagination-ellipsis">&hellip;</span></li>`)
			}
			if p == page {
				s.WriteString(fmt.Sprintf(`<li><a class="pagination-link is-current">%d</a></li>`, p))
			} else {
				s.WriteString(fmt.Sprintf(`<li><a class="pagination-link" href="%s">%d</a></li>`, pageLink(p), p))
			}
			lastShown = p
		}
		s.WriteString(`</ul></nav>`)
	}

	s.WriteString(`</div>`)
	return s.String()
}
