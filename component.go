package dbexplorer

import (
	"net/url"
	"slices"
)

// A Component is a part of an application that owns some tables outright
// and publishes contract views for other components to read. Each table
// has at most one owner.
type Component struct {
	Name   string
	Tables []string // tables the component owns
	Views  []string // contract views it publishes
}

// A Catalog tells the explorer which components a database is divided
// into. With one set, the explorer can be scoped to a single component
// (/c/{component}/…) as well as showing everything.
type Catalog interface {
	Components() []Component
}

// StaticCatalog is a Catalog fixed in code.
type StaticCatalog []Component

// Components returns the catalog itself.
func (c StaticCatalog) Components() []Component { return c }

// component returns the named component of the catalog.
func (e *Explorer) component(name string) (Component, bool) {
	if e.Catalog == nil {
		return Component{}, false
	}
	for _, c := range e.Catalog.Components() {
		if c.Name == name {
			return c, true
		}
	}
	return Component{}, false
}

// owner returns the component owning a table or publishing a view, or "".
func (e *Explorer) owner(name string) string {
	if e.Catalog == nil {
		return ""
	}
	for _, c := range e.Catalog.Components() {
		if slices.Contains(c.Tables, name) || slices.Contains(c.Views, name) {
			return c.Name
		}
	}
	return ""
}

// scopeFor is the scope a link from scope cur to table should land in: the
// same scope when the table belongs to it (or nothing is scoped), otherwise
// the table's owner — or everything, for an unowned table.
func (e *Explorer) scopeFor(cur, table string) string {
	if cur == "" {
		return ""
	}
	return e.owner(table)
}

// scopeBase is the URL prefix of a scope: BasePath for everything,
// BasePath/c/{component} for one component.
func (e *Explorer) scopeBase(component string) string {
	if component == "" {
		return e.BasePath
	}
	return e.BasePath + "/c/" + url.PathEscape(component)
}

// indexURLIn is the index page of a scope.
func (e *Explorer) indexURLIn(component string) string {
	if component == "" {
		return e.indexURL()
	}
	return e.scopeBase(component)
}

// tableURLIn is a table's page within a scope.
func (e *Explorer) tableURLIn(component, name string) string {
	return e.scopeBase(component) + "/" + url.PathEscape(name)
}
