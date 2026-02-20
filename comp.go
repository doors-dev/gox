package gox

import (
	"context"
	"io"
)

// Comp is the minimal component interface in GoX.
//
// A component produces its content by returning an Elem from Main.
// Main may return nil to render nothing.
type Comp interface {
	Main() Elem
}

// Elem is the fundamental renderable value in GoX.
//
// Elem is a function that emits rendering jobs through the provided Cursor.
// Most generated `.gox` output ultimately compiles to one or more Elem values.
//
// Elem implements Comp (Main returns itself) and also implements Templ-compatible
// rendering via Render(ctx, w).
type Elem func(cur Cursor) error

// Main makes Elem satisfy Comp by returning itself.
//
// If e is nil, Main returns nil.
func (e Elem) Main() Elem { return e }

// Print renders e by streaming jobs to printer using ctx as the default context.
//
// Print creates a new Cursor bound to the given context and printer, then executes
// the Elem function.
//
// If e is nil, Print returns nil.
func (e Elem) Print(ctx context.Context, printer Printer) error {
	if e == nil {
		return nil
	}
	cur := NewCursor(ctx, printer)
	return e(cur)
}

// Render renders e into w using GoX’s default Printer implementation.
//
// Render is provided for interoperability with templ-style renderers.
// If e is nil, Render returns nil.
func (e Elem) Render(ctx context.Context, w io.Writer) error {
	if e == nil {
		return nil
	}
	printer := NewPrinter(w)
	return e.Print(ctx, printer)
}

// Proxy can intercept and transform an element subtree before it is rendered.
//
// Proxy implementations are invoked with the current Cursor and the target Elem.
// A proxy may emit its own jobs, modify cursor state via Editor-like operations,
// wrap/replace the element, or render it conditionally.
type Proxy interface {
	Proxy(cur Cursor, elem Elem) error
}

// Templ is the minimal interface for templ-compatible components.
//
// Elem implements Templ via Render(ctx, w).
type Templ interface {
	Render(ctx context.Context, w io.Writer) error
}

// Editor performs advanced rendering by operating directly on a Cursor.
//
// Editor is an escape hatch for low-level control (for example, emitting custom
// jobs or performing cursor-driven edits).
type Editor interface {
	Edit(cur Cursor) error
}

var _ Comp = Elem(nil)
var _ Templ = Elem(nil)
