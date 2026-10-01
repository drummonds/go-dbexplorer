package dbexplorer_test

import (
	"strings"
	"testing"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
)

// The test library split into two components: the catalogue owns authors
// and books and publishes contract_books; lending owns loans, whose book_id
// references the catalogue's books.
var libraryCatalog = dbexplorer.StaticCatalog{
	{Name: "catalogue", Tables: []string{"authors", "books"}, Views: []string{"contract_books"}},
	{Name: "lending", Tables: []string{"loans"}},
}

// componentBackends is backends with the library catalog and its contract
// view in place.
func componentBackends(t *testing.T) map[string]*dbexplorer.Explorer {
	t.Helper()
	m := backends(t)
	for _, e := range m {
		if _, err := e.DB.Exec(`CREATE VIEW contract_books AS SELECT id, title FROM books`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { e.DB.Exec(`DROP VIEW contract_books`) })
		e.Catalog = libraryCatalog
	}
	return m
}

func TestAllComponentsIndex(t *testing.T) {
	for backend, e := range componentBackends(t) {
		t.Run(backend, func(t *testing.T) {
			index := e.Render(t.Context(), "/")
			for _, want := range []string{
				`href="/c/catalogue"`, `href="/c/lending"`, // the scopes
				`href="/books">books</a>`, // tables still browse unscoped
			} {
				if !strings.Contains(index, want) {
					t.Errorf("index missing %q", want)
				}
			}
			if strings.Count(index, `href="/c/catalogue"`) < 3 {
				t.Error("authors and books should each link their owner, catalogue")
			}
		})
	}
}

func TestComponentIndex(t *testing.T) {
	for backend, e := range componentBackends(t) {
		t.Run(backend, func(t *testing.T) {
			index := e.Render(t.Context(), "/c/catalogue")
			for _, want := range []string{`href="/c/catalogue/authors"`, `href="/c/catalogue/books"`, `href="/c/catalogue/contract_books"`, "catalogue"} {
				if !strings.Contains(index, want) {
					t.Errorf("catalogue index missing %q", want)
				}
			}
			if strings.Contains(index, ">loans</a>") {
				t.Error("catalogue index lists lending's loans")
			}
			lending := e.Render(t.Context(), "/c/lending")
			if !strings.Contains(lending, `href="/c/lending/loans"`) || strings.Contains(lending, ">authors</a>") || strings.Contains(lending, ">contract_books</a>") {
				t.Errorf("lending index should list only loans: %.500s", lending)
			}
			if !strings.Contains(e.Render(t.Context(), "/c/nope"), "Unknown component") {
				t.Error("an unknown component should say so")
			}
		})
	}
}

func TestComponentTable(t *testing.T) {
	for backend, e := range componentBackends(t) {
		t.Run(backend, func(t *testing.T) {
			books := e.Render(t.Context(), "/c/catalogue/books?sort=title&dir=asc")
			for _, want := range []string{
				"Table: books",
				`href="/c/catalogue"`, // back to the component, not everything
				`href="/c/catalogue/authors?filter=id&amp;value=1"`, // FK within the component stays scoped
				`href="/c/catalogue/books?dir=desc&amp;sort=title"`, // sort links stay scoped
			} {
				if !strings.Contains(books, want) {
					t.Errorf("scoped books page missing %q", want)
				}
			}
			loans := e.Render(t.Context(), "/c/lending/loans")
			if !strings.Contains(loans, `href="/c/catalogue/books?filter=id&amp;value=1"`) {
				t.Error("an FK into another component should link into that component's scope")
			}
			if !strings.Contains(e.Render(t.Context(), "/c/lending/books"), "Table Not Found") {
				t.Error("a table outside the component should not be browsable in its scope")
			}
		})
	}
}

// With no catalog there are no components: /c/... is just an unknown table.
func TestNoCatalogNoScopes(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			if strings.Contains(e.Render(t.Context(), "/"), `href="/c/`) {
				t.Error("index links component scopes without a catalog")
			}
			if !strings.Contains(e.Render(t.Context(), "/c/catalogue"), "Table Not Found") {
				t.Error("/c/... without a catalog should be an unknown table")
			}
		})
	}
}
