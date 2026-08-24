package gox

import (
	"context"
	"io"

	"github.com/doors-dev/gox/internal/utils"
)

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
//
// Writes replace &, <, >, " and ' with entities and NUL with U+FFFD, and pass
// everything else through. That covers element text and quoted attribute
// values; it is not sufficient for JavaScript, CSS, or URL contexts.
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
