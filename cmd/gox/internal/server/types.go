package server

import (
	"context"
	"io"
)

type Listener interface {
	Accept() (io.ReadWriteCloser, error)
	Close() error
}

type Dialer interface {
	Dial(context.Context) (io.ReadWriteCloser, error)
}

type Server interface {
	Wait()
}
