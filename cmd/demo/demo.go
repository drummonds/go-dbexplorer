package main

import (
	"context"
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

// library is the sample database's components: the catalogue owns the
// books and their authors and publishes contract_books; lending owns the
// members and their loans, whose book_id reaches into the catalogue.
var library = dbexplorer.StaticCatalog{
	{Name: "catalogue", Tables: []string{"authors", "books"}, Views: []string{"contract_books"}},
	{Name: "lending", Tables: []string{"members", "loans"}},
}

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
		bulma: &dbexplorer.Explorer{DB: db, UUIDLen: uuidLen, TimeFormat: timeFormat, Catalog: library},
		plain: &dbexplorer.Explorer{DB: db, UUIDLen: uuidLen, TimeFormat: timeFormat, Catalog: library,
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
func (d *demo) Render(ctx context.Context, rawURL string) string {
	return d.explorerFor(rawURL).Render(ctx, rawURL)
}
