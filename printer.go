package gox

import (
	"context"
	"io"

	"github.com/doors-dev/gox/internal/utils"
)

// Printer consumes Jobs produced during rendering.
//
// A Printer defines how the job stream is handled. The default implementation
// created by NewPrinter writes job output sequentially to an io.Writer, but
// alternative implementations may buffer, transform, parallelize, or analyze
// jobs before producing final output.
//
// Printer is not required to be safe for concurrent use unless an implementation
// explicitly documents otherwise.
type Printer interface {
	Send(j Job) error
}

// Output is the low-level output interface used by Jobs.
//
// Implementations write their representation to an io.Writer.
// Output is aliased from an internal utility type.
type Output = utils.Output

// Job is a unit of rendering work emitted by Cursor and consumed by a Printer.
//
// Each job carries a context and an Output implementation. Printers may use the
// context for cancellation/deadlines and for propagating errors through the
// rendering pipeline.
type Job interface {
	// Context returns the context associated with this job.
	Context() context.Context
	Output
}

type printer struct {
	w io.Writer
}

// Send renders j to the underlying writer.
//
// The default printer checks j.Context().Err() before rendering and returns that
// error if the context is canceled or expired. Otherwise it calls j.Output(w).
func (p *printer) Send(j Job) error {
	if j.Context().Err() != nil {
		return j.Context().Err()
	}
	return j.Output(p.w)
}

// NewPrinter returns the default Printer implementation that writes job output
// sequentially to w.
func NewPrinter(w io.Writer) Printer {
	return &printer{w: w}
}
