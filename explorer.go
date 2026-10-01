// Package dbexplorer renders a lightweight HTML database explorer — table
// catalog, per-table schema, foreign keys, indexes, and a paginated,
// sortable data browser — for any database opened through database/sql.
//
// Metadata comes from the PostgreSQL catalogs (pg_tables, pg_views,
// information_schema, pg_indexes), which real PostgreSQL and pglike both
// provide. Pages render through a Skin of html/templates over exported view
// models; the built-in skin uses Bulma CSS class names and is unstyled
// without it.
package dbexplorer

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Explorer renders explorer pages for one database.
type Explorer struct {
	DB *sql.DB
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
	// Skin is the set of templates pages render through (see ParseSkin).
	// Nil uses the built-in Bulma skin.
	Skin *Skin
	// Catalog, when set, divides the database into components and lets the
	// explorer be scoped to one of them (see Render).
	Catalog Catalog
	// Authoriser, when set, decides which components the viewer may see;
	// nil shows everything. The viewer comes from the context passed to
	// Render (or the request's, under Handler).
	Authoriser Authoriser
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
	// Component scopes the page to one component of the Catalog: only its
	// tables and views are browsable, and links stay in the scope unless
	// they lead to another component's table. Empty means everything.
	Component string
}

func (e *Explorer) indexURL() string {
	if e.BasePath == "" {
		return "/"
	}
	return e.BasePath
}

// Render renders the page for a URL relative to BasePath: the index for ""
// or "/", otherwise the table page for "/<table>". With a Catalog,
// "/c/<component>" and "/c/<component>/<table>" are the same pages scoped
// to one component. The query string supplies page, sort, dir, trunc,
// filter and value (see TableOptions). ctx is handed to the Authoriser to
// identify the viewer. This is the routing-free core used by Handler and by
// WASM hosts that dispatch navigation themselves.
func (e *Explorer) Render(ctx context.Context, rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return e.IndexHTML(ctx)
	}
	path := strings.Trim(strings.TrimPrefix(u.Path, e.BasePath), "/")
	component := ""
	if rest, ok := strings.CutPrefix(path, "c/"); ok && e.Catalog != nil {
		component, path, _ = strings.Cut(rest, "/")
	}
	if path == "" {
		return e.render("index", e.indexView(ctx, component))
	}
	q := u.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	return e.TableHTMLWith(ctx, path, TableOptions{
		Page: page, Sort: q.Get("sort"), Dir: q.Get("dir"), Trunc: q.Get("trunc") == "1",
		FilterCol: q.Get("filter"), FilterVal: q.Get("value"), Component: component,
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
		fmt.Fprint(w, e.render("page", PageView{
			Title:   title,
			Content: template.HTML(e.Render(r.Context(), r.URL.String())),
			Footer:  template.HTML(e.Footer),
		}))
	})
}

// The explorer reads catalog metadata through the helpers below, using the
// PostgreSQL catalogs for the public schema.

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
	rows, err := e.DB.Query(`SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
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
	rows, err := e.DB.Query(`SELECT viewname FROM pg_views WHERE schemaname = 'public' ORDER BY viewname`)
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

// tableColumns returns the columns of a table in definition order.
func (e *Explorer) tableColumns(name string) []dbColInfo {
	if e.DB == nil {
		return nil
	}
	var cols []dbColInfo
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

// tableFKs returns the foreign keys declared on a table.
func (e *Explorer) tableFKs(name string) []dbFKInfo {
	if e.DB == nil {
		return nil
	}
	var fks []dbFKInfo
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

// tableIndexes returns the indexes on a table.
func (e *Explorer) tableIndexes(name string) []dbIdxInfo {
	if e.DB == nil {
		return nil
	}
	var idxs []dbIdxInfo
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

// IndexHTML renders the explorer overview: all tables with row/column
// counts and foreign-key relationships.
func (e *Explorer) IndexHTML(ctx context.Context) string {
	return e.render("index", e.indexView(ctx, ""))
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
		return e.tableURLIn(o.Component, name)
	}
	return e.tableURLIn(o.Component, name) + "?" + q.Encode()
}

// TableHTML renders the detail view for a single table: schema, foreign
// keys, indexes, and paginated data. It is TableHTMLWith without a filter;
// trunc shortens text cells to truncLen characters.
func (e *Explorer) TableHTML(ctx context.Context, name string, page int, sort string, dir string, trunc bool) string {
	return e.TableHTMLWith(ctx, name, TableOptions{Page: page, Sort: sort, Dir: dir, Trunc: trunc})
}

// TableHTMLWith renders the detail view for a single table — schema with
// foreign keys and indexes, then the paginated, sortable, optionally
// filtered data — as an HTML fragment. Sort, pagination and filter state is
// carried through every emitted link. Cells in foreign-key columns link to
// the referenced table filtered to that key.
func (e *Explorer) TableHTMLWith(ctx context.Context, name string, o TableOptions) string {
	return e.render("table", e.tableView(ctx, name, o))
}
