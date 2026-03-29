package gox

import (
	"context"
	"io"

	"github.com/doors-dev/gox/internal/utils"
)

// EditorComp is both a low-level Editor and a regular Comp.
//
// It is useful for values that should plug into component-based APIs while
// still exposing direct cursor control.
type EditorComp interface {
	Editor
	Comp
}

// EditorCompFunc adapts a function into an EditorComp.
//
// It is useful for small helpers that need direct cursor access but should also
// satisfy component-based APIs.
type EditorCompFunc func(cur Cursor) error

// Edit calls e(cur).
func (e EditorCompFunc) Edit(cur Cursor) error {
	return e(cur)
}

// Main makes EditorCompFunc satisfy Comp by returning it as an Elem.
func (e EditorCompFunc) Main() Elem {
	return Elem(e)
}

// EditorFunc adapts a function into an Editor.
type EditorFunc func(cur Cursor) error

// Edit calls e(cur).
func (e EditorFunc) Edit(cur Cursor) error {
	return e(cur)
}

var _ Editor = EditorFunc(nil)

// ProxyFunc adapts a function into a Proxy.
type ProxyFunc func(cur Cursor, elem Elem) error

// Proxy calls p(cur, elem).
func (p ProxyFunc) Proxy(cur Cursor, elem Elem) error {
	return p(cur, elem)
}

var _ Proxy = ProxyFunc(nil)


// NewEscapedWriter returns a writer that applies GoX's HTML escaping rules.
//
// It is useful in custom printers or helpers that need the same escaping
// behavior as Text, Fprint, and attribute output.
func NewEscapedWriter(w io.Writer) io.Writer {
	return utils.NewEscapedWriter(w)
}

// AttrModFunc adapts a function into a Modify.
type AttrModFunc func(ctx context.Context, tag string, attrs Attrs) error

// Modify calls a(ctx, tag, attrs).
func (a AttrModFunc) Modify(ctx context.Context, tag string, attrs Attrs) error {
	return a(ctx, tag, attrs)
}

var _ Modify = AttrModFunc(nil)

// PrinterFunc adapts a function into a Printer.
type PrinterFunc func(j Job) error

// Send calls p(j).
func (p PrinterFunc) Send(j Job) error {
	return p(j)
}

var _ Printer = PrinterFunc(nil)
