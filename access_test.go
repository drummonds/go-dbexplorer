package dbexplorer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
)

// viewerKey carries the components a test viewer may see.
type viewerKey struct{}

func asViewer(ctx context.Context, components ...string) context.Context {
	return context.WithValue(ctx, viewerKey{}, components)
}

// byViewer allows the components listed in the context; "" stands for
// unowned tables.
var byViewer = dbexplorer.AuthoriserFunc(func(ctx context.Context, component string) bool {
	allowed, _ := ctx.Value(viewerKey{}).([]string)
	return slices.Contains(allowed, component)
})

func TestDeniedComponentHiddenFromIndex(t *testing.T) {
	for backend, e := range componentBackends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Authoriser = byViewer
			ctx := asViewer(t.Context(), "catalogue")
			index := e.Render(ctx, "/")
			if !strings.Contains(index, ">books</a>") || !strings.Contains(index, `href="/c/catalogue"`) {
				t.Error("catalogue should be visible")
			}
			if strings.Contains(index, ">loans</a>") || strings.Contains(index, `href="/c/lending"`) {
				t.Error("lending's tables and scope should be hidden")
			}
		})
	}
}

func TestDeniedPagesSayDenied(t *testing.T) {
	for backend, e := range componentBackends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Authoriser = byViewer
			ctx := asViewer(t.Context(), "catalogue")
			for _, u := range []string{"/c/lending", "/c/lending/loans", "/loans"} {
				page := e.Render(ctx, u)
				if !strings.Contains(page, "Access denied") || strings.Contains(page, "Data (") {
					t.Errorf("%s should be denied: %.300s", u, page)
				}
			}
		})
	}
}

// A foreign key into a component the viewer may not see shows its value
// and the owner, but no link.
func TestForeignKeyIntoDeniedComponent(t *testing.T) {
	for backend, e := range componentBackends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Authoriser = byViewer
			loans := e.Render(asViewer(t.Context(), "lending"), "/c/lending/loans")
			if !strings.Contains(loans, "Table: loans") {
				t.Fatalf("lending viewer should see loans: %.300s", loans)
			}
			if strings.Contains(loans, `href="/c/catalogue/books`) {
				t.Error("book_id should not link into the denied catalogue")
			}
			if !strings.Contains(loans, `<td>1 <span class="tag is-light">catalogue</span></td>`) {
				t.Error("book_id should show its value tagged with the owner")
			}
		})
	}
}

// Without a catalog every table is unowned, so the authoriser is a single
// gate asked about component "".
func TestAuthoriserWithoutCatalog(t *testing.T) {
	for backend, e := range backends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Authoriser = byViewer
			if page := e.Render(asViewer(t.Context()), "/books"); !strings.Contains(page, "Access denied") {
				t.Error("a viewer without \"\" should be denied unowned tables")
			}
			if page := e.Render(asViewer(t.Context(), ""), "/books"); !strings.Contains(page, "Data (3 rows)") {
				t.Error("a viewer with \"\" should see unowned tables")
			}
		})
	}
}

// Handler asks the authoriser with the request's context.
func TestHandlerUsesRequestContext(t *testing.T) {
	for backend, e := range componentBackends(t) {
		t.Run(backend, func(t *testing.T) {
			e.Authoriser = byViewer
			req := httptest.NewRequest(http.MethodGet, "/c/lending/loans", nil)
			rec := httptest.NewRecorder()
			e.Handler().ServeHTTP(rec, req.WithContext(asViewer(req.Context(), "lending")))
			if !strings.Contains(rec.Body.String(), "Table: loans") {
				t.Error("Handler should authorise with the request context")
			}
		})
	}
}
