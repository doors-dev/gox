package gox

import (
	"context"
	"io"
)

type Printer interface {
	Send(j Job) error
}

type Job interface {
	Context() context.Context
	Output(w io.Writer) error
}

type printer struct {
	w io.Writer
}

func (p *printer) Send(j Job) error {
	if j.Context().Err() != nil {
		return j.Context().Err()
	}
	return j.Output(p.w)
}

func NewPrinter(w io.Writer) Printer {
	return &printer{w: w}
}
