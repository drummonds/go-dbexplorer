//go:build !(js && wasm)

// The go-dbexplorer demo server. With no DSN it explores an in-memory
// pglike (SQLite) database seeded with a sample lending library — the same
// data the WASM demo shows in the browser. Point it at real PostgreSQL with
// -dsn or DBEXPLORER_PG_DSN to explore that instead (the sample data is
// seeded there too if the tables are absent). -uuid-len and -time-format
// set the explorer's column formatting.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"html"
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
	uuidLen := flag.Int("uuid-len", 8, "show UUIDs shortened to this many characters (0 = full)")
	timeFormat := flag.String("time-format", "2006-01-02 15:04", "Go layout for timestamp columns (empty = Go default)")
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

	ver := buildVersion()
	ex := &dbexplorer.Explorer{
		DB:         db,
		UUIDLen:    *uuidLen,
		TimeFormat: *timeFormat,
		Title:      "go-dbexplorer demo " + ver + " — " + backend,
		Footer: fmt.Sprintf(`go-dbexplorer demo %s &middot; %s &middot; <a href="https://git.bytestone.uk/hum3/go-dbexplorer">Source</a>`,
			html.EscapeString(ver), html.EscapeString(backend)),
	}
	http.Handle("/", ex.Handler())

	log.Printf("go-dbexplorer demo %s (%s) on http://localhost%s/", ver, backend, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
