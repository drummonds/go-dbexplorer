//go:build !(js && wasm)

// The go-dbexplorer demo server. With no DSN it explores an in-memory
// pglike (SQLite) database seeded with a sample lending library — the same
// data the WASM demo shows in the browser. Point it at real PostgreSQL with
// -dsn or DBEXPLORER_PG_DSN to explore that instead (the sample data is
// seeded there too if the tables are absent).
package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
	_ "git.bytestone.uk/hum3/go-postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dsn := flag.String("dsn", os.Getenv("DBEXPLORER_PG_DSN"),
		"postgres:// DSN for real PostgreSQL; empty = in-memory pglike")
	seed := flag.Bool("seed", true, "seed the sample library data if the tables are absent")
	flag.Parse()

	var db *sql.DB
	var err error
	backend := "pglike (in-memory SQLite)"
	if *dsn == "" {
		db, err = sql.Open("pglike", "file::memory:?_pragma=temp_store(2)")
	} else {
		backend = "PostgreSQL"
		db, err = sql.Open("pgx", *dsn)
	}
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if *dsn != "" {
		if err := db.Ping(); err != nil {
			log.Fatalf("cannot reach %s: %v", *dsn, err)
		}
	}
	if *seed {
		seedSampleDB(db)
	}

	ex := &dbexplorer.Explorer{
		DB:       db,
		Postgres: *dsn != "",
		Title:    "go-dbexplorer demo — " + backend,
	}
	http.Handle("/", ex.Handler())

	log.Printf("go-dbexplorer demo (%s) on http://localhost%s/", backend, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
