package dbexplorer_test

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
	_ "git.bytestone.uk/hum3/go-postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func seedTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE authors (id SERIAL PRIMARY KEY, name TEXT NOT NULL)`,
		`CREATE TABLE books (id SERIAL PRIMARY KEY, title TEXT NOT NULL,
			author_id INTEGER NOT NULL REFERENCES authors(id))`,
		`CREATE INDEX idx_books_author ON books(author_id)`,
		`INSERT INTO authors (name) VALUES ('Ursula K. Le Guin')`,
		`INSERT INTO books (title, author_id) VALUES ('The Dispossessed', 1)`,
		`INSERT INTO books (title, author_id) VALUES ('The Left Hand of Darkness', 1)`,
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
		for _, tbl := range []string{"books", "authors"} {
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
