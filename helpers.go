package gox

import (
	"io"

	"github.com/doors-dev/gox/internal/utils"
)

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
