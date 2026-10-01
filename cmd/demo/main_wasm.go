//go:build js && wasm

// WASM build of the demo: pglike (SQLite compiled to WASM) runs the sample
// library database entirely in the browser. The page's JS intercepts link
// clicks and calls goRender(url) for each navigation (URLs under /plain
// render in the plain skin); goVersion() reports
// the build version.
package main

import (
	"database/sql"
	"log"
	"syscall/js"

	_ "git.bytestone.uk/hum3/go-postgres"
)

func main() {
	db, err := sql.Open("pglike", "file::memory:?_pragma=temp_store(2)")
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	seedSampleDB(db)

	d := newDemo(db, 8, "2006-01-02 15:04")

	js.Global().Set("goRender", js.FuncOf(func(this js.Value, args []js.Value) any {
		url := "/"
		if len(args) > 0 {
			url = args[0].String()
		}
		return d.Render(url)
	}))

	js.Global().Set("goVersion", js.FuncOf(func(this js.Value, args []js.Value) any {
		return buildVersion()
	}))

	select {} // keep the Go runtime alive for goRender/goVersion calls
}
