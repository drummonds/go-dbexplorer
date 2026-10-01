package dbexplorer

import (
	"fmt"
	"html/template"
	"slices"
)

// The view models below are what a skin's templates render. They carry
// display-ready data — formatted cell text, every link already built — so a
// template only arranges it. Fields are part of the public API: a skin
// written against them keeps working across releases of the same major
// version.

// Link is a piece of text and the URL it points to.
type Link struct {
	Text, URL string
}

// IndexView is rendered by the "index" template: the tables and views of
// the database, and the foreign-key relationships between tables.
type IndexView struct {
	Error         string // set when the database is unavailable
	Tables, Views []TableSummary
	Relationships []Relationship
}

// TableSummary is one table or view in the index.
type TableSummary struct {
	Name, URL   string
	Annotation  template.HTML // the host's Annotate fragment, trusted HTML
	Columns     int
	Rows        int
	CountFailed bool // the row count query failed; Rows is meaningless
}

// Relationship is one foreign key, from a column of one table to a column
// of another.
type Relationship struct {
	From, To             Link
	FromColumn, ToColumn string
}

// TableView is rendered by the "table" template: one table's schema, keys
// and indexes, then a page of its data.
type TableView struct {
	Name     string
	IndexURL string
	NotFound bool   // no table or view has this name
	Error    string // database unavailable, or the data query failed

	Columns     []Column
	ForeignKeys []ForeignKey
	Indexes     []TableIndex

	TotalRows   int     // rows matching the filter, across all pages
	Filter      *Filter // nil when the data is unfiltered
	Trunc       bool    // text cells are shortened
	TruncToggle string  // URL of this page with Trunc flipped
	Headers     []Header
	Rows        [][]Cell
	Pager       *Pager // nil when the data fits on one page
}

// Column is one column of a table's schema.
type Column struct {
	Position            int
	Name, Type, Default string
	NotNull, PrimaryKey bool
}

// ForeignKey is a foreign key declared on the table.
type ForeignKey struct {
	Column             string
	References         Link // the referenced table
	ReferencedColumn   string
	OnUpdate, OnDelete string
}

// TableIndex is an index on the table.
type TableIndex struct {
	Name, Columns string
	Unique        bool
	Origin        string // "pk" for the primary-key index, else ""
}

// Filter is the column = value restriction on the data, with the URL that
// removes it.
type Filter struct {
	Column, Value, ClearURL string
}

// Header is a data column heading; URL sorts by it (toggling the direction
// when it is already the sort column).
type Header struct {
	Name, URL string
	Sorted    string // "asc" or "desc" when this is the sort column, else ""
}

// Cell is one data value. When Full is set, Text is a shortened form of it
// and Full belongs in a tooltip. URL links a foreign-key value to the
// referenced row.
type Cell struct {
	Null            bool
	Text, Full, URL string
}

// Pager links the pages of the data. PrevURL and NextURL are empty at the
// ends; Links holds the first, last and neighbouring pages.
type Pager struct {
	Page, Pages      int
	PrevURL, NextURL string
	Links            []PageLink
}

// PageLink is one numbered page link. Gap marks a run of skipped pages
// before it.
type PageLink struct {
	Number  int
	URL     string
	Current bool
	Gap     bool
}

const pageSize = 50

func (e *Explorer) indexView() IndexView {
	if e.DB == nil {
		return IndexView{Error: "Database not available."}
	}
	var v IndexView
	for _, name := range e.Tables() {
		v.Tables = append(v.Tables, e.summary(name))
		for _, f := range e.tableFKs(name) {
			v.Relationships = append(v.Relationships, Relationship{
				From: Link{name, e.tableURL(name)}, FromColumn: f.fromCol,
				To: Link{f.refTable, e.tableURL(f.refTable)}, ToColumn: f.toCol,
			})
		}
	}
	for _, name := range e.Views() {
		v.Views = append(v.Views, e.summary(name))
	}
	return v
}

func (e *Explorer) summary(name string) TableSummary {
	s := TableSummary{Name: name, URL: e.tableURL(name), Columns: len(e.tableColumns(name))}
	if e.Annotate != nil {
		s.Annotation = template.HTML(e.Annotate(name))
	}
	if err := e.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, name)).Scan(&s.Rows); err != nil {
		s.CountFailed = true
	}
	return s
}

func (e *Explorer) tableView(name string, o TableOptions) TableView {
	v := TableView{Name: name, IndexURL: e.indexURL()}
	if e.DB == nil {
		v.Error = "Database not available."
		return v
	}
	if !slices.Contains(e.Tables(), name) && !slices.Contains(e.Views(), name) {
		v.NotFound = true
		return v
	}

	var colNames []string
	for _, c := range e.tableColumns(name) {
		colNames = append(colNames, c.name)
		v.Columns = append(v.Columns, Column{Position: c.pos, Name: c.name, Type: c.ctype,
			Default: c.dflt, NotNull: c.notNull, PrimaryKey: c.pk})
	}
	fks := e.tableFKs(name)
	fkByCol := map[string]dbFKInfo{}
	for _, f := range fks {
		fkByCol[f.fromCol] = f
		v.ForeignKeys = append(v.ForeignKeys, ForeignKey{Column: f.fromCol,
			References: Link{f.refTable, e.tableURL(f.refTable)}, ReferencedColumn: f.toCol,
			OnUpdate: f.onUpdate, OnDelete: f.onDelete})
	}
	for _, ix := range e.tableIndexes(name) {
		v.Indexes = append(v.Indexes, TableIndex{Name: ix.name, Columns: ix.columns, Unique: ix.unique, Origin: ix.origin})
	}

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

	e.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"%s`, name, where), args...).Scan(&v.TotalRows)
	totalPages := max((v.TotalRows+pageSize-1)/pageSize, 1)
	page := min(max(o.Page, 1), totalPages)

	// cur is the effective state every link on this page is derived from.
	cur := TableOptions{Page: page, Sort: sort, Dir: dir, Trunc: o.Trunc, FilterCol: filterCol, FilterVal: filterVal}

	if filterCol != "" {
		clear := cur
		clear.Page, clear.FilterCol, clear.FilterVal = 1, "", ""
		v.Filter = &Filter{Column: filterCol, Value: filterVal, ClearURL: e.tableLink(name, clear)}
	}
	toggle := cur
	toggle.Trunc = !o.Trunc
	v.Trunc, v.TruncToggle = o.Trunc, e.tableLink(name, toggle)

	query := fmt.Sprintf(`SELECT * FROM "%s"%s`, name, where)
	if sort != "" {
		query += fmt.Sprintf(` ORDER BY "%s" %s`, sort, dir)
	}
	query += fmt.Sprintf(` LIMIT %d OFFSET %d`, pageSize, (page-1)*pageSize)
	rows, err := e.DB.Query(query, args...)
	if err != nil {
		v.Error = fmt.Sprintf("Query error: %v", err)
		return v
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	for _, col := range cols {
		hdr := cur
		hdr.Sort, hdr.Dir = col, "asc"
		h := Header{Name: col}
		if col == sort {
			h.Sorted = dir
			if dir == "asc" {
				hdr.Dir = "desc"
			}
		}
		h.URL = e.tableLink(name, hdr)
		v.Headers = append(v.Headers, h)
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if rows.Scan(ptrs...) != nil {
			continue
		}
		row := make([]Cell, len(cols))
		for i, val := range vals {
			row[i] = e.cell(name, cols[i], val, o.Trunc)
			if f, ok := fkByCol[cols[i]]; ok && val != nil {
				row[i].URL = e.tableLink(f.refTable, TableOptions{Trunc: o.Trunc, FilterCol: f.toCol, FilterVal: rawText(val)})
			}
		}
		v.Rows = append(v.Rows, row)
	}

	if totalPages > 1 {
		v.Pager = e.pager(name, cur, totalPages)
	}
	return v
}

// cell formats one value: Format (if set) first, else the built-in
// UUID/time/truncation formatting.
func (e *Explorer) cell(table, column string, v any, trunc bool) Cell {
	if v == nil {
		return Cell{Null: true}
	}
	if e.Format != nil {
		if text, ok := e.Format(table, column, v); ok {
			return Cell{Text: text}
		}
	}
	full, short := e.formatValue(v, trunc)
	if short != "" {
		return Cell{Text: short, Full: full}
	}
	return Cell{Text: full}
}

// pager links the first, last, current and neighbouring pages, carrying
// sort, trunc and filter in every link.
func (e *Explorer) pager(name string, cur TableOptions, totalPages int) *Pager {
	page := cur.Page
	link := func(p int) string {
		pp := cur
		pp.Page = p
		return e.tableLink(name, pp)
	}
	pg := &Pager{Page: page, Pages: totalPages}
	if page > 1 {
		pg.PrevURL = link(page - 1)
	}
	if page < totalPages {
		pg.NextURL = link(page + 1)
	}
	last := 0
	for _, p := range []int{1, page - 1, page, page + 1, totalPages} {
		if p <= last || p < 1 || p > totalPages {
			continue
		}
		pg.Links = append(pg.Links, PageLink{Number: p, URL: link(p), Current: p == page, Gap: last > 0 && p > last+1})
		last = p
	}
	return pg
}
