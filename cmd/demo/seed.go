package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand/v2"
)

// newUUIDs returns n version-4-shaped UUIDs from a fixed-seed generator, so
// the demo shows the same ids on every run.
func newUUIDs(rng *rand.Rand, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		var b [16]byte
		for j := range b {
			b[j] = byte(rng.UintN(256))
		}
		b[6] = (b[6] & 0x0f) | 0x40 // version 4
		b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
		ids[i] = fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	}
	return ids
}

// seedSampleDB creates and fills a small lending-library schema — enough
// tables, foreign keys and indexes to show every explorer feature. All keys
// are UUIDs, generated deterministically here (the DEFAULT covers rows
// added by hand; pglike accepts the bare PostgreSQL form since go-postgres
// v0.5.12). Safe to call on a database where the tables already exist (it
// skips seeding).
func seedSampleDB(db *sql.DB) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM books`).Scan(&n); err == nil && n > 0 {
		return // already seeded
	}

	ddl := []string{
		`CREATE TABLE IF NOT EXISTS authors (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			born INTEGER)`,
		`CREATE TABLE IF NOT EXISTS members (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			joined TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS books (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title TEXT NOT NULL,
			author_id UUID NOT NULL REFERENCES authors(id),
			published INTEGER,
			isbn VARCHAR(17))`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_books_isbn ON books(isbn)`,
		`CREATE INDEX IF NOT EXISTS idx_books_author ON books(author_id)`,
		`CREATE TABLE IF NOT EXISTS loans (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			book_id UUID NOT NULL REFERENCES books(id),
			member_id UUID NOT NULL REFERENCES members(id),
			borrowed TIMESTAMP NOT NULL,
			returned TIMESTAMP)`,
		`CREATE INDEX IF NOT EXISTS idx_loans_book ON loans(book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_loans_member ON loans(member_id)`,
	}
	for _, s := range ddl {
		if _, err := db.Exec(s); err != nil {
			log.Printf("seed ddl: %v", err)
			return
		}
	}
	// The catalogue component's contract view: books as other components
	// may read them. Plain CREATE VIEW (pglike has no OR REPLACE, PostgreSQL
	// no IF NOT EXISTS), so a view left from an earlier seed only logs.
	if _, err := db.Exec(`CREATE VIEW contract_books AS SELECT id, title, author_id, isbn FROM books`); err != nil {
		log.Printf("seed view: %v", err)
	}

	rng := rand.New(rand.NewPCG(20260902, 1))

	authors := []struct {
		name string
		born int
	}{
		{"Ursula K. Le Guin", 1929}, {"Iain M. Banks", 1954}, {"Octavia E. Butler", 1947},
		{"Terry Pratchett", 1948}, {"Ann Leckie", 1966}, {"Stanisław Lem", 1921},
		{"Connie Willis", 1945}, {"Gene Wolfe", 1931},
	}
	authorIDs := newUUIDs(rng, len(authors))
	for i, a := range authors {
		if _, err := db.Exec(`INSERT INTO authors (id, name, born) VALUES ($1, $2, $3)`,
			authorIDs[i], a.name, a.born); err != nil {
			log.Printf("seed authors: %v", err)
			return
		}
	}

	books := []struct {
		title  string
		author int // 1-based index into authors
		year   int
	}{
		{"The Dispossessed", 1, 1974}, {"The Left Hand of Darkness", 1, 1969},
		{"A Wizard of Earthsea", 1, 1968}, {"The Player of Games", 2, 1988},
		{"Use of Weapons", 2, 1990}, {"Excession", 2, 1996},
		{"Kindred", 3, 1979}, {"Parable of the Sower", 3, 1993},
		{"Small Gods", 4, 1992}, {"Night Watch", 4, 2002}, {"Going Postal", 4, 2004},
		{"Ancillary Justice", 5, 2013}, {"Ancillary Sword", 5, 2014},
		{"Solaris", 6, 1961}, {"The Cyberiad", 6, 1965},
		{"Doomsday Book", 7, 1992}, {"To Say Nothing of the Dog", 7, 1997},
		{"The Shadow of the Torturer", 8, 1980}, {"The Claw of the Conciliator", 8, 1981},
	}
	bookIDs := newUUIDs(rng, len(books))
	for i, b := range books {
		isbn := fmt.Sprintf("978-0-000-%05d-%d", i+1, (i*7)%10)
		if _, err := db.Exec(`INSERT INTO books (id, title, author_id, published, isbn) VALUES ($1, $2, $3, $4, $5)`,
			bookIDs[i], b.title, authorIDs[b.author-1], b.year, isbn); err != nil {
			log.Printf("seed books: %v", err)
			return
		}
	}

	const nMembers = 12
	memberIDs := newUUIDs(rng, nMembers)
	for i, id := range memberIDs {
		if _, err := db.Exec(`INSERT INTO members (id, name, joined) VALUES ($1, $2, $3)`,
			id, fmt.Sprintf("Member %02d", i+1),
			fmt.Sprintf("2025-%02d-01 10:00:00", (i%12)+1)); err != nil {
			log.Printf("seed members: %v", err)
			return
		}
	}

	// Deterministic pseudo-random loans; some still out (returned IS NULL).
	const nLoans = 120
	loanIDs := newUUIDs(rng, nLoans)
	for i := range nLoans {
		book := bookIDs[(i*13)%len(books)]
		member := memberIDs[(i*7)%nMembers]
		day := (i % 27) + 1
		borrowed := fmt.Sprintf("2026-%02d-%02d 14:00:00", (i%12)+1, day)
		var returned any
		if i%5 != 0 {
			returned = fmt.Sprintf("2026-%02d-%02d 09:30:00", (i%12)+1, day+2)
		}
		if _, err := db.Exec(`INSERT INTO loans (id, book_id, member_id, borrowed, returned) VALUES ($1, $2, $3, $4, $5)`,
			loanIDs[i], book, member, borrowed, returned); err != nil {
			log.Printf("seed loans: %v", err)
			return
		}
	}
}
