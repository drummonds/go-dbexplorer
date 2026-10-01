package dbexplorer

import "context"

// An Authoriser decides which components a viewer may see. The explorer
// does not know who the viewer is: the host puts that in the context it
// passes to Render (Handler passes the request's context) and the
// Authoriser reads it back.
//
// Tables no component owns — every table, when there is no Catalog — are
// asked about as component "".
type Authoriser interface {
	CanViewComponent(ctx context.Context, component string) bool
}

// AuthoriserFunc adapts a function to an Authoriser.
type AuthoriserFunc func(ctx context.Context, component string) bool

// CanViewComponent calls f.
func (f AuthoriserFunc) CanViewComponent(ctx context.Context, component string) bool {
	return f(ctx, component)
}

// canView reports whether the viewer may see a component; with no
// Authoriser everything is visible.
func (e *Explorer) canView(ctx context.Context, component string) bool {
	return e.Authoriser == nil || e.Authoriser.CanViewComponent(ctx, component)
}

// canViewTable reports whether the viewer may see a table or view, by its
// owner.
func (e *Explorer) canViewTable(ctx context.Context, name string) bool {
	return e.canView(ctx, e.owner(name))
}
