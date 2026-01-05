package gox

import (
	"context"
	"io"
)

type Comp interface {
	Main() Elem
}

type Elem func(ctx context.Context, cursor Cursor) error

func (e Elem) Main() Elem {
	return e
}

func (e Elem) Print(ctx context.Context, printer Printer) error {
	cursor := NewCursor(printer)
	defer cursor.terminate()
	return e(ctx, cursor)
}

func (e Elem) Render(ctx context.Context, w io.Writer) error {
	printer := NewPrinter(w)
	return e.Print(ctx, printer)
}

type Templ interface {
	Render(ctx context.Context, w io.Writer) error
}

var _ Comp = Elem(nil)
var _ Templ = Elem(nil)
