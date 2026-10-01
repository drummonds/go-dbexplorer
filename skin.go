package dbexplorer

import (
	"bytes"
	"embed"
	"html"
	"html/template"
	"io/fs"
)

//go:embed skins/bulma/*.tmpl
var bulmaFS embed.FS

// A Skin is the set of templates the explorer renders through:
//
//   - "index" executes with an IndexView
//   - "table" executes with a TableView
//   - "page" executes with a PageView and wraps the content for Handler
//
// The built-in skin emits Bulma class names. Templates are html/template, so
// every value is escaped unless the view model types it template.HTML.
type Skin struct {
	t *template.Template
}

// PageView is rendered by the "page" template: a complete HTML document
// around one explorer page.
type PageView struct {
	Title   string
	Content template.HTML // the rendered index or table page
	Footer  template.HTML // Explorer.Footer, trusted HTML
}

// ParseSkin builds a skin from the *.tmpl files at the root of fsys,
// layered over the built-in Bulma skin: a template fsys defines replaces the
// built-in one of the same name, and any it leaves out are kept. A nil fsys
// gives the built-in skin. Errors in the templates are reported here, not
// when a page is rendered.
func ParseSkin(fsys fs.FS) (*Skin, error) {
	t, err := template.ParseFS(bulmaFS, "skins/bulma/*.tmpl")
	if err != nil {
		return nil, err
	}
	if fsys != nil {
		if t, err = t.ParseFS(fsys, "*.tmpl"); err != nil {
			return nil, err
		}
	}
	return &Skin{t: t}, nil
}

var bulmaSkin = mustSkin(ParseSkin(nil))

func mustSkin(s *Skin, err error) *Skin {
	if err != nil {
		panic(err)
	}
	return s
}

// render executes the named template of the explorer's skin. A template
// that fails at run time yields an error message instead of a partial page.
func (e *Explorer) render(name string, data any) string {
	skin := e.Skin
	if skin == nil {
		skin = bulmaSkin
	}
	var b bytes.Buffer
	if err := skin.t.ExecuteTemplate(&b, name, data); err != nil {
		return "<p>Template error: " + html.EscapeString(err.Error()) + "</p>"
	}
	return b.String()
}
