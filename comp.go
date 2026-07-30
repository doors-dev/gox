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

// Print sends e to printer as a single root JobComp carrying ctx.
//
// Print does not render e: the printer receives exactly one job and nothing
// runs until the printer acts on it. Calling Job.Output on that job renders
// the subtree through a fresh default Printer and Cursor writing to Output's
// writer, so the nested jobs never reach the custom printer; a custom printer
// that needs those jobs must expand the component itself, by running
// Comp.Main's Elem against a Cursor bound to that printer instead of calling
// Output. If e is nil, Print sends nothing and returns nil.
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
// Generated code for `~>(p) <div>...</div>` calls p.Proxy(cur, el), where el
// holds the wrapped subtree. Rendering el is the implementation's
// responsibility: a Proxy that returns without calling el(cur) (or
// cur.Comp(el)) drops the subtree, and running el against a different Cursor
// is what reroutes it. The error Proxy returns becomes the render error.
//
// Proxies are useful when a subtree needs cross-cutting behavior such as
// instrumentation, attribute injection, conditional rendering, or rerouting
// through a custom Printer.
type Proxy interface {
	Proxy(cur Cursor, el Elem) error
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
