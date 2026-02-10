package gox

import (
	"context"
	"io"

	"github.com/doors-dev/gox/utils"
)

type Printer interface {
	Send(j Job) error
}

type Output = utils.Output

type Job interface {
	Context() context.Context
	Output
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
