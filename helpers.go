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

func (e EditorCompFunc) Edit(cur Cursor) error {
	return e(cur)
}

func (e EditorCompFunc) Main() Elem {
	return Elem(e)
}

// EditorFunc adapts a function into an Editor.
type EditorFunc func(cur Cursor) error

func (e EditorFunc) Edit(cur Cursor) error {
	return e(cur)
}

var _ Editor = EditorFunc(nil)

// ProxyFunc adapts a function into a Proxy.
type ProxyFunc func(cur Cursor, el Elem) error

func (p ProxyFunc) Proxy(cur Cursor, el Elem) error {
	return p(cur, el)
}

var _ Proxy = ProxyFunc(nil)

// NewEscapedWriter returns a writer that applies GoX's HTML escaping rules.
//
// It is useful in custom printers or helpers that need the same escaping
// behavior as Text, Fprint, and attribute output.
func NewEscapedWriter(w io.Writer) io.Writer {
	return utils.NewEscapedWriter(w)
}

// ModifyFunc adapts a function into a Modify.
type ModifyFunc func(ctx context.Context, tag string, attrs Attrs) error

func (a ModifyFunc) Modify(ctx context.Context, tag string, attrs Attrs) error {
	return a(ctx, tag, attrs)
}

var _ Modify = ModifyFunc(nil)

// PrinterFunc adapts a function into a Printer.
type PrinterFunc func(j Job) error

func (p PrinterFunc) Send(j Job) error {
	return p(j)
}

var _ Printer = PrinterFunc(nil)
