package dbexplorer_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
)

// A skin overrides only the templates it defines; the rest stay the
// built-in Bulma ones.
func TestSkinOverridesTemplates(t *testing.T) {
	skin, err := dbexplorer.ParseSkin(fstest.MapFS{
		"index.tmpl": {Data: []byte(`{{define "index"}}<ul>{{range .Tables}}<li class="custom"><a href="{{.URL}}">{{.Name}}</a> {{.Rows}}</li>{{end}}</ul>{{end}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Skin = skin
			defer func() { e.Skin = nil }()
			index := e.IndexHTML(t.Context())
			if !strings.Contains(index, `<li class="custom"><a href="/books">books</a> 3</li>`) {
				t.Errorf("index not rendered by the skin: %.300s", index)
			}
			if strings.Contains(index, `class="box"`) {
				t.Error("index still carries the Bulma markup")
			}
			if table := e.TableHTML(t.Context(), "books", 1, "", "", false); !strings.Contains(table, `<details class="box">`) {
				t.Error("table page should fall back to the built-in template")
			}
		})
	}
}

// Handler wraps the content in the skin's page template.
func TestSkinPageTemplate(t *testing.T) {
	skin, err := dbexplorer.ParseSkin(fstest.MapFS{
		"page.tmpl": {Data: []byte(`{{define "page"}}<html><title>{{.Title}}</title><main>{{.Content}}</main><small>{{.Footer}}</small></html>{{end}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Skin, e.Title, e.Footer = skin, "Lib <1>", "<b>v1</b>"
			defer func() { e.Skin, e.Title, e.Footer = nil, "", "" }()
			rec := httptest.NewRecorder()
			e.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/books", nil))
			body := rec.Body.String()
			for _, want := range []string{"<title>Lib &lt;1&gt;</title>", "<main><h2", "Table: books", "<small><b>v1</b></small>"} {
				if !strings.Contains(body, want) {
					t.Errorf("page missing %q: %.300s", want, body)
				}
			}
		})
	}
}

// A broken skin fails when it is parsed, not on the first request.
func TestParseSkinRejectsBrokenTemplate(t *testing.T) {
	_, err := dbexplorer.ParseSkin(fstest.MapFS{
		"index.tmpl": {Data: []byte(`{{define "index"}}{{range .Tables}}{{end}`)},
	})
	if err == nil {
		t.Fatal("ParseSkin accepted a malformed template")
	}
}

// Host-supplied text is escaped by every skin; only Annotate and Footer are
// trusted HTML.
func TestSkinEscapesData(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Format = func(table, column string, v any) (string, bool) {
				if column == "title" {
					return "<script>x</script>", true
				}
				return "", false
			}
			defer func() { e.Format = nil }()
			page := e.TableHTML(t.Context(), "books", 1, "", "", false)
			if strings.Contains(page, "<script>x</script>") || !strings.Contains(page, "&lt;script&gt;x&lt;/script&gt;") {
				t.Error("formatted cell text not escaped")
			}
		})
	}
}
