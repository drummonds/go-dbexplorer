package main

import (
	"database/sql"
	"fmt"
	"log"
)

// seedSampleDB creates and fills a small lending-library schema — enough
// tables, foreign keys and indexes to show every explorer feature. Safe to
// call on a database where the tables already exist (it skips seeding).
func seedSampleDB(db *sql.DB) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM books`).Scan(&n); err == nil && n > 0 {
		return // already seeded
	}

	ddl := []string{
		`CREATE TABLE IF NOT EXISTS authors (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			born INTEGER)`,
		`CREATE TABLE IF NOT EXISTS members (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			joined TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS books (
			id SERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			author_id INTEGER NOT NULL REFERENCES authors(id),
			published INTEGER,
			isbn VARCHAR(17))`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_books_isbn ON books(isbn)`,
		`CREATE INDEX IF NOT EXISTS idx_books_author ON books(author_id)`,
		`CREATE TABLE IF NOT EXISTS loans (
			id SERIAL PRIMARY KEY,
			book_id INTEGER NOT NULL REFERENCES books(id),
			member_id INTEGER NOT NULL REFERENCES members(id),
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

	authors := []struct {
		name string
		born int
	}{
		{"Ursula K. Le Guin", 1929}, {"Iain M. Banks", 1954}, {"Octavia E. Butler", 1947},
		{"Terry Pratchett", 1948}, {"Ann Leckie", 1966}, {"Stanisław Lem", 1921},
		{"Connie Willis", 1945}, {"Gene Wolfe", 1931},
	}
	for _, a := range authors {
		if _, err := db.Exec(`INSERT INTO authors (name, born) VALUES ($1, $2)`, a.name, a.born); err != nil {
			log.Printf("seed authors: %v", err)
			return
		}
	}

	books := []struct {
		title  string
		author int
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
	for i, b := range books {
		isbn := fmt.Sprintf("978-0-000-%05d-%d", i+1, (i*7)%10)
		if _, err := db.Exec(`INSERT INTO books (title, author_id, published, isbn) VALUES ($1, $2, $3, $4)`,
			b.title, b.author, b.year, isbn); err != nil {
			log.Printf("seed books: %v", err)
			return
		}
	}

	for i := 1; i <= 12; i++ {
		if _, err := db.Exec(`INSERT INTO members (name, joined) VALUES ($1, $2)`,
			fmt.Sprintf("Member %02d", i),
			fmt.Sprintf("2025-%02d-01 10:00:00", (i%12)+1)); err != nil {
			log.Printf("seed members: %v", err)
			return
		}
	}

	// Deterministic pseudo-random loans; some still out (returned IS NULL).
	for i := range 120 {
		book := (i*13)%len(books) + 1
		member := (i*7)%12 + 1
		day := (i % 27) + 1
		borrowed := fmt.Sprintf("2026-%02d-%02d 14:00:00", (i%12)+1, day)
		if i%5 == 0 {
			if _, err := db.Exec(`INSERT INTO loans (book_id, member_id, borrowed) VALUES ($1, $2, $3)`,
				book, member, borrowed); err != nil {
				log.Printf("seed loans: %v", err)
				return
			}
		} else {
			returned := fmt.Sprintf("2026-%02d-%02d 09:30:00", (i%12)+1, day+2)
			if _, err := db.Exec(`INSERT INTO loans (book_id, member_id, borrowed, returned) VALUES ($1, $2, $3, $4)`,
				book, member, borrowed, returned); err != nil {
				log.Printf("seed loans: %v", err)
				return
			}
		}
	}
}
