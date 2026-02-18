package conn

import (
	"context"
	"io"

	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
)

func NewConn(ctx context.Context, rwc io.ReadWriteCloser) *Conn {
	framer := jsonrpc2.HeaderFramer()
	c := &Conn{
		ctx:    ctx,
		ch:     make(chan jsonrpc2.Message, 1),
		reader: framer.Reader(rwc),
		writer: framer.Writer(rwc),
		closer: rwc,
	}
	c.launch()
	return c
}

type Conn struct {
	ctx    context.Context
	ch     chan jsonrpc2.Message
	reader jsonrpc2.Reader
	writer jsonrpc2.Writer
	closer io.Closer
}

func (c *Conn) launch() {
	go c.wait()
	go c.read()
}

func (c *Conn) wait() {
	<-c.ctx.Done()
	c.closer.Close()
}

func (c *Conn) read() {
	defer close(c.ch)
	for {
		m, err := c.reader.Read(c.ctx)
		if err != nil {
			return
		}
		select {
		case <-c.ctx.Done():
			return
		case c.ch <- m:
		}
	}
}

func (c *Conn) Read() <-chan jsonrpc2.Message {
	return c.ch
}

func (c *Conn) Write(m jsonrpc2.Message) error {
	return c.writer.Write(c.ctx, m)
}
