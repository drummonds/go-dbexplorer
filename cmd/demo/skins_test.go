package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "git.bytestone.uk/hum3/go-postgres"
)

func demoDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pglike", "file::memory:?_pragma=temp_store(2)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	seedSampleDB(db)
	return db
}

// URLs under /plain go to the plain-skinned explorer, the rest to Bulma.
func TestDemoRoutesBySkin(t *testing.T) {
	d := newDemo(demoDB(t), 8, "2006-01-02 15:04")
	if got := d.explorerFor("/plain/books"); got != d.plain {
		t.Error("/plain/books should use the plain explorer")
	}
	if got := d.explorerFor("/plain"); got != d.plain {
		t.Error("/plain should use the plain explorer")
	}
	if got := d.explorerFor("/books"); got != d.bulma {
		t.Error("/books should use the Bulma explorer")
	}
	if got := d.explorerFor("/plainly"); got != d.bulma {
		t.Error("/plainly is a table name, not the plain skin")
	}
}

// The plain skin restructures the page, not just its styling: data rows
// are records, and its links stay inside the plain explorer.
func TestPlainSkin(t *testing.T) {
	d := newDemo(demoDB(t), 8, "2006-01-02 15:04")
	index := d.Render(t.Context(), "/plain")
	if !strings.Contains(index, `href="/plain/books"`) || strings.Contains(index, `class="box"`) {
		t.Errorf("plain index: %.400s", index)
	}
	books := d.Render(t.Context(), "/plain/books?sort=title&dir=asc")
	for _, want := range []string{"<article", "<dl>", `href="/plain/authors?filter=id&amp;value=`, "Ancillary Justice"} {
		if !strings.Contains(books, want) {
			t.Errorf("plain books page missing %q", want)
		}
	}
	if loans := d.Render(t.Context(), "/plain/loans?page=2"); !strings.Contains(loans, "Page 2 of 3") || !strings.Contains(loans, `href="/plain/loans?page=3"`) {
		t.Error("plain loans page should page through 120 rows")
	}
	if strings.Contains(books, `class="table`) {
		t.Error("plain books page carries Bulma markup")
	}
	if !strings.Contains(d.Render(t.Context(), "/books"), `class="box"`) {
		t.Error("the default explorer should keep the Bulma skin")
	}
}

// The demo library is two components: the catalogue (authors, books, and
// the contract_books view it publishes) and lending (members, loans).
// Both skins browse it per component.
func TestDemoComponents(t *testing.T) {
	d := newDemo(demoDB(t), 8, "2006-01-02 15:04")
	for _, base := range []string{"", plainBase} {
		index := d.Render(t.Context(), base+"/")
		if !strings.Contains(index, `href="`+base+`/c/catalogue"`) || !strings.Contains(index, `href="`+base+`/c/lending"`) {
			t.Errorf("%s index does not link the components", base)
		}
		catalogue := d.Render(t.Context(), base+"/c/catalogue")
		for _, want := range []string{">authors</a>", ">books</a>", ">contract_books</a>"} {
			if !strings.Contains(catalogue, want) {
				t.Errorf("%s catalogue scope missing %q", base, want)
			}
		}
		if strings.Contains(catalogue, ">loans</a>") {
			t.Errorf("%s catalogue scope lists loans", base)
		}
		if loans := d.Render(t.Context(), base+"/c/lending/loans"); !strings.Contains(loans, `href="`+base+`/c/catalogue/books?filter=id&amp;value=`) {
			t.Errorf("%s loans' book_id should link into the catalogue scope", base)
		}
	}
}
