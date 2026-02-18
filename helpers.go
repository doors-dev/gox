package gox

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
