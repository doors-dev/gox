package gox

import (
	"context"
	"io"
)

type Comp interface {
	Main() Elem
}

type Elem func(ctx context.Context, cur Cursor) error

func (e Elem) Main() Elem {
	return e
}

func (e Elem) Print(ctx context.Context, printer Printer) error {
	cur := NewCursor(ctx, printer)
	defer cur.terminate()
	return e(ctx, cur)
}

func (e Elem) Render(ctx context.Context, w io.Writer) error {
	printer := NewPrinter(w)
	return e.Print(ctx, printer)
}

type Proxy interface {
	Proxy(ctx context.Context, cur Cursor, elem Elem) error
}

type Templ interface {
	Render(ctx context.Context, w io.Writer) error
}

var _ Comp = Elem(nil)
var _ Templ = Elem(nil)
