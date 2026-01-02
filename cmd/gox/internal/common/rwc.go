package common

import "io"

func RWClose(r io.Reader, w io.Writer, close func() error) io.ReadWriteCloser {
	return rwc{
		r: r,
		w: w,
		c: close,
	}
}

type rwc struct {
	r io.Reader
	w io.Writer
	c func() error
}

func (w rwc) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

func (w rwc) Read(p []byte) (n int, err error) {
	return w.r.Read(p)
}

func (w rwc) Close() error {
	return w.c()
}
