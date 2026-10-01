package main

import (
	"database/sql"
	"embed"
	"io/fs"
	"strings"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
)

// The plain skin is how a host writes its own: templates over the
// explorer's view models, here classless HTML for simple.css with data rows
// as records rather than a table.
//
//go:embed skins/plain/*.tmpl
var plainFS embed.FS

// plainBase is where the plain-skinned explorer is mounted.
const plainBase = "/plain"

// demo is the same database explored through two skins: the built-in
// Bulma skin at /, and the plain skin at /plain.
type demo struct {
	bulma, plain *dbexplorer.Explorer
}

func newDemo(db *sql.DB, uuidLen int, timeFormat string) *demo {
	sub, err := fs.Sub(plainFS, "skins/plain")
	if err != nil {
		panic(err)
	}
	skin, err := dbexplorer.ParseSkin(sub)
	if err != nil {
		panic(err) // the embedded skin is fixed at build time; tests catch a bad one
	}
	return &demo{
		bulma: &dbexplorer.Explorer{DB: db, UUIDLen: uuidLen, TimeFormat: timeFormat},
		plain: &dbexplorer.Explorer{DB: db, UUIDLen: uuidLen, TimeFormat: timeFormat,
			BasePath: plainBase, Skin: skin},
	}
}

// explorerFor picks the explorer a URL belongs to.
func (d *demo) explorerFor(rawURL string) *dbexplorer.Explorer {
	path, _, _ := strings.Cut(rawURL, "?")
	if path == plainBase || strings.HasPrefix(path, plainBase+"/") {
		return d.plain
	}
	return d.bulma
}

// Render renders a URL in whichever explorer it belongs to.
func (d *demo) Render(rawURL string) string {
	return d.explorerFor(rawURL).Render(rawURL)
}
