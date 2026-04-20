package gox

import (
	"context"
	"io"
)

// Comp is anything that can produce a root Elem.
//
// In practice, most components are written in `.gox` as `elem (T) Main() { ... }`.
// Elem itself also satisfies Comp, so APIs can accept either components or
// plain Elem values through one interface. Main may return nil to render
// nothing.
type Comp interface {
	Main() Elem
}

// Elem is the runtime value produced by GoX template syntax.
//
// In practice, Elem is usually authored in `.gox` as an HTML expression or with
// the `elem` form.
//
// Example:
//
//	var badge gox.Elem = <span class="badge">New</span>
//
//	elem Badge(label string) {
//		<span class="badge">~(label)</span>
//	}
//
// Generated code lowers Elem to Cursor operations. Elem also implements Comp
// and templ-style rendering through Render.
type Elem func(cur Cursor) error

// Main makes Elem satisfy Comp by returning itself.
func (e Elem) Main() Elem { return e }

// Print sends e through printer as a root component job using ctx as the job context.
func (e Elem) Print(ctx context.Context, printer Printer) error {
	if e == nil {
		return nil
	}
	job := NewJobComp(ctx, e)
	return printer.Send(job)
}

// Render writes e to w with GoX's default Printer.
//
// Render is the usual way to turn an Elem into HTML and also makes Elem usable
// in templ-style rendering pipelines. If e is nil, Render returns nil.
func (e Elem) Render(ctx context.Context, w io.Writer) error {
	if e == nil {
		return nil
	}
	printer := NewPrinter(w)
	cur := NewCursor(ctx, printer)
	return e(cur)
}

// Proxy wraps an Elem before it is rendered.
//
// Proxies are useful when a subtree needs cross-cutting behavior such as
// instrumentation, attribute injection, conditional rendering, or rerouting
// through a custom Printer.
type Proxy interface {
	Proxy(cur Cursor, elem Elem) error
}

// Templ is the minimal templ-compatible rendering interface.
//
// Elem implements Templ via Render.
type Templ interface {
	Render(ctx context.Context, w io.Writer) error
}

// Editor renders by operating on a Cursor directly.
//
// Use Editor when returning another Elem is not enough and the implementation
// needs low-level access to cursor methods or custom jobs.
type Editor interface {
	Edit(cur Cursor) error
}

var _ Comp = Elem(nil)
var _ Templ = Elem(nil)
