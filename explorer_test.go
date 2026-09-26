package dbexplorer_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
	_ "git.bytestone.uk/hum3/go-postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const testUUID = "0f8fad5b-d9cb-469f-a165-70867728950e"

func seedTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE authors (id SERIAL PRIMARY KEY, name TEXT NOT NULL)`,
		`CREATE TABLE books (id SERIAL PRIMARY KEY, title TEXT NOT NULL,
			author_id INTEGER NOT NULL REFERENCES authors(id))`,
		`CREATE INDEX idx_books_author ON books(author_id)`,
		`CREATE TABLE loans (id UUID PRIMARY KEY, book_id INTEGER NOT NULL REFERENCES books(id),
			borrowed TIMESTAMP NOT NULL, returned TIMESTAMP)`,
		`INSERT INTO authors (name) VALUES ('Ursula K. Le Guin')`,
		`INSERT INTO authors (name) VALUES ('Iain M. Banks')`,
		`INSERT INTO books (title, author_id) VALUES ('The Dispossessed', 1)`,
		`INSERT INTO books (title, author_id) VALUES ('The Left Hand of Darkness', 1)`,
		`INSERT INTO books (title, author_id) VALUES ('Excession', 2)`,
		`INSERT INTO loans (id, book_id, borrowed) VALUES ('` + testUUID + `', 1, '2026-01-05 14:30:00')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed %q: %v", s[:min(len(s), 40)], err)
		}
	}
}

// backends returns the explorer under pglike always, and under real
// PostgreSQL too when DBEXPLORER_PG_DSN is set.
func backends(t *testing.T) map[string]*dbexplorer.Explorer {
	t.Helper()
	m := map[string]*dbexplorer.Explorer{}

	pglike, err := sql.Open("pglike", "file::memory:?_pragma=temp_store(2)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pglike.Close() })
	seedTestDB(t, pglike)
	m["pglike"] = &dbexplorer.Explorer{DB: pglike}

	if dsn := os.Getenv("DBEXPLORER_PG_DSN"); dsn != "" {
		pg, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pg.Close() })
		for _, tbl := range []string{"loans", "books", "authors"} {
			pg.Exec(`DROP TABLE IF EXISTS ` + tbl + ` CASCADE`)
		}
		seedTestDB(t, pg)
		m["postgres"] = &dbexplorer.Explorer{DB: pg, Postgres: true}
	}
	return m
}

func TestIndexHTML(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			html := e.IndexHTML()
			for _, want := range []string{"authors", "books", "Relationships", "author_id"} {
				if !strings.Contains(html, want) {
					t.Errorf("index missing %q", want)
				}
			}
		})
	}
}

func TestTableHTML(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			html := e.TableHTML("books", 1, "title", "asc", false)
			for _, want := range []string{"Table: books", "Schema", "Foreign Keys", "Indexes",
				"The Dispossessed", "idx_books_author", "authors"} {
				if !strings.Contains(html, want) {
					t.Errorf("table page missing %q", want)
				}
			}
			// Schema is a closed master <details>; FKs and indexes nest inside it.
			if strings.Contains(html, "<details open") {
				t.Error("detail sections should be closed by default")
			}
			schemaAt := strings.Index(html, "Schema (")
			fkAt := strings.Index(html, "Foreign Keys (")
			idxAt := strings.Index(html, "Indexes (")
			dataAt := strings.Index(html, "Data (")
			if schemaAt < 0 || fkAt < 0 || idxAt < 0 || dataAt < 0 ||
				!(schemaAt < fkAt && fkAt < idxAt && idxAt < dataAt) {
				t.Errorf("expected Schema > Foreign Keys > Indexes before Data, got offsets %d %d %d %d",
					schemaAt, fkAt, idxAt, dataAt)
			}
			masterAt := strings.LastIndex(html[:schemaAt], "<details")
			if masterAt < 0 || strings.Count(html[masterAt:dataAt], "<details") != 3 ||
				strings.Count(html[masterAt:dataAt], "</details>") != 3 {
				t.Error("expected schema master plus two nested detail sections")
			}
			if !strings.Contains(e.TableHTML("nope", 1, "", "", false), "Table Not Found") {
				t.Error("unknown table should render Table Not Found")
			}
			// Injection attempt via sort must be rejected by column validation.
			if got := e.TableHTML("books", 1, `title";DROP TABLE books;--`, "asc", false); strings.Contains(got, "Query error") {
				t.Error("invalid sort column should fall back, not error")
			}
		})
	}
}

func TestRenderRouting(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			if !strings.Contains(e.Render("/"), "DB Explorer") {
				t.Error("Render(/) should be the index")
			}
			if !strings.Contains(e.Render("/books?page=1&sort=title&dir=asc"), "Table: books") {
				t.Error("Render(/books) should be the table page")
			}
			e.BasePath = "/internal/explorer"
			defer func() { e.BasePath = "" }()
			out := e.Render("/internal/explorer/books")
			if !strings.Contains(out, "Table: books") {
				t.Error("Render should strip BasePath")
			}
			if !strings.Contains(out, `href="/internal/explorer/authors"`) {
				t.Error("links should carry BasePath")
			}
		})
	}
}

func TestHandlerTitleAndFooter(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Title = "Demo <v1>"
			e.Footer = `build <b>v1</b>`
			rec := httptest.NewRecorder()
			e.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			body := rec.Body.String()
			if !strings.Contains(body, "<title>Demo &lt;v1&gt;</title>") {
				t.Error("title should be escaped in <title>")
			}
			if !strings.Contains(body, `<footer class="mt-5 is-size-7 has-text-grey">build <b>v1</b></footer>`) {
				t.Error("footer should be rendered verbatim below the content")
			}
			if strings.Index(body, "DB Explorer") > strings.Index(body, "<footer") {
				t.Error("footer should come after the content")
			}
		})
	}
}

func TestCellFormatting(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			// Defaults: full UUID, Go's default time text, NULL in grey.
			out := e.TableHTML("loans", 1, "", "", false)
			if !strings.Contains(out, "<td>"+testUUID+"</td>") {
				t.Error("UUID should be shown in full when UUIDLen is 0")
			}
			if !strings.Contains(out, `<td class="has-text-grey-light">NULL</td>`) {
				t.Error("NULL should render as grey NULL")
			}
			if strings.Contains(out, "<nil>") {
				t.Error("NULL must not render as <nil>")
			}

			e.UUIDLen = 8
			e.TimeFormat = "02 Jan 2006 15:04"
			out = e.TableHTML("loans", 1, "", "", false)
			if !strings.Contains(out, fmt.Sprintf(`<td title="%s">%s&hellip;</td>`, testUUID, testUUID[:8])) {
				t.Errorf("UUID should be shortened to 8 chars with full value in title:\n%s", out)
			}
			if !strings.Contains(out, "<td>05 Jan 2026 14:30</td>") {
				t.Errorf("TIMESTAMP should honour TimeFormat:\n%s", out)
			}

			e.Format = func(table, column string, v any) (string, bool) {
				if table == "loans" && column == "borrowed" {
					return "custom", true
				}
				return "", false
			}
			out = e.TableHTML("loans", 1, "", "", false)
			if !strings.Contains(out, "<td>custom</td>") || strings.Contains(out, "05 Jan 2026") {
				t.Error("Format hook should take precedence for its column")
			}
			if !strings.Contains(out, testUUID[:8]+"&hellip;") {
				t.Error("Format hook returning false should fall through to built-in formatting")
			}
			e.UUIDLen, e.TimeFormat, e.Format = 0, "", nil
		})
	}
}

func TestForeignKeyLinksAndFilter(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			out := e.TableHTML("books", 1, "", "", false)
			if !strings.Contains(out, `<td><a href="/authors?filter=id&value=1">1</a></td>`) {
				t.Errorf("FK cell should link to the referenced table filtered on the key:\n%s", out)
			}
			if strings.Contains(out, `<td><a href="/authors?filter=id&value=`) &&
				strings.Count(out, `href="/authors?filter=id&value=1"`) != 2 {
				t.Error("both Le Guin books should link to author 1")
			}

			filtered := e.TableHTMLWith("books", dbexplorer.TableOptions{FilterCol: "author_id", FilterVal: "2"})
			if !strings.Contains(filtered, "Data (1 rows)") || !strings.Contains(filtered, "Excession") ||
				strings.Contains(filtered, "The Dispossessed") {
				t.Errorf("filter author_id=2 should show only Excession:\n%s", filtered)
			}
			if !strings.Contains(filtered, "Filter: author_id = 2") || !strings.Contains(filtered, `href="/books">clear filter</a>`) {
				t.Error("filtered page should show the filter and a clear link")
			}
			// Sort and toggle links keep the filter.
			if !strings.Contains(filtered, `href="/books?dir=asc&filter=author_id&sort=title&value=2">title</a>`) {
				t.Errorf("sort links should carry the filter:\n%s", filtered)
			}
			if !strings.Contains(filtered, `href="/books?filter=author_id&trunc=1&value=2"`) {
				t.Error("truncate toggle should carry the filter")
			}

			// Unknown filter column is ignored; injection via the column is impossible.
			all := e.TableHTMLWith("books", dbexplorer.TableOptions{FilterCol: `id" OR 1=1 --`, FilterVal: "x"})
			if !strings.Contains(all, "Data (3 rows)") || strings.Contains(all, "Filter:") {
				t.Error("unknown filter column should be ignored")
			}
			// Filter value is a parameter, not SQL.
			none := e.TableHTMLWith("books", dbexplorer.TableOptions{FilterCol: "title", FilterVal: "' OR 1=1 --"})
			if !strings.Contains(none, "Data (0 rows)") || strings.Contains(none, "Query error") {
				t.Errorf("filter value must be parameterised:\n%s", none)
			}

			// Render wires filter/value from the query string.
			viaURL := e.Render("/books?filter=author_id&value=2")
			if !strings.Contains(viaURL, "Data (1 rows)") {
				t.Error("Render should apply filter/value query params")
			}
		})
	}
}

// Views are listed and browsable alongside tables, and an Annotate hook
// lets the host label each object (an owner, a contract badge) in the
// index.
func TestViewsListedAndAnnotated(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			if _, err := e.DB.Exec(`CREATE VIEW contract_books AS SELECT b.title, a.name AS author FROM books b JOIN authors a ON a.id = b.author_id`); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { e.DB.Exec(`DROP VIEW contract_books`) })
			e.Annotate = func(name string) string {
				if strings.HasPrefix(name, "contract_") {
					return `<span class="tag">contract</span>`
				}
				return `<span class="tag">internal</span>`
			}
			if views := e.Views(); len(views) != 1 || views[0] != "contract_books" {
				t.Fatalf("Views() = %v, want [contract_books]", views)
			}
			index := e.IndexHTML()
			for _, want := range []string{"contract_books", `<span class="tag">contract</span>`, `<span class="tag">internal</span>`} {
				if !strings.Contains(index, want) {
					t.Errorf("index missing %q", want)
				}
			}
			page := e.TableHTML("contract_books", 1, "", "", false)
			for _, want := range []string{"Excession", "author"} {
				if !strings.Contains(page, want) {
					t.Errorf("view page missing %q", want)
				}
			}
		})
	}
}
