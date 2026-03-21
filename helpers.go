package gox

import (
	"context"
	"io"

	"github.com/doors-dev/gox/internal/utils"
)

type EditorComp interface {
	Editor
	Comp
}

type EditorCompFunc func(cur Cursor) error

func (e EditorCompFunc) Edit(cur Cursor) error {
	return e(cur)
}

func (e EditorCompFunc) Main() Elem {
	return Elem(e)
}

type EditorFunc func(cur Cursor) error

func (e EditorFunc) Edit(cur Cursor) error {
	return e(cur)
}

var _ Editor = EditorFunc(nil)

type ProxyFunc func(cur Cursor, elem Elem) error

func (p ProxyFunc) Proxy(cur Cursor, elem Elem) error {
	return p(cur, elem)
}

var _ Proxy = ProxyFunc(nil)

func Noop(any) {}

func NewEscapedWriter(w io.Writer) io.Writer {
	return utils.NewEscapedWriter(w)
}

type AttrModFunc func(ctx context.Context, tag string, attrs Attrs) error

func (a AttrModFunc) Modify(ctx context.Context, tag string, attrs Attrs) error {
	return a(ctx, tag, attrs)
}

var _ Modify = AttrModFunc(nil)

type PrinterFunc func(j Job) error

func (p PrinterFunc) Send(j Job) error {
	return p(j)
}

var _ Printer = PrinterFunc(nil)
