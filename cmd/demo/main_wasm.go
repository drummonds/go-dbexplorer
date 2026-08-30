//go:build js && wasm

// WASM build of the demo: pglike (SQLite compiled to WASM) runs the sample
// library database entirely in the browser. The page's JS intercepts link
// clicks and calls goRender(url) for each navigation.
package main

import (
	"database/sql"
	"log"
	"syscall/js"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
	_ "git.bytestone.uk/hum3/go-postgres"
)

func main() {
	db, err := sql.Open("pglike", "file::memory:?_pragma=temp_store(2)")
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	seedSampleDB(db)

	ex := &dbexplorer.Explorer{DB: db}

	js.Global().Set("goRender", js.FuncOf(func(this js.Value, args []js.Value) any {
		url := "/"
		if len(args) > 0 {
			url = args[0].String()
		}
		return ex.Render(url)
	}))

	select {} // keep the Go runtime alive for goRender calls
}
