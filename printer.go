package gox

import (
	"context"
	"io"

	"github.com/doors-dev/gox/internal/utils"
)

// Printer consumes the Job stream produced during rendering.
//
// The default Printer from NewPrinter writes HTML sequentially to an io.Writer.
// Custom printers can inspect, buffer, rewrite, or reroute jobs before final
// output. Printer implementations are not required to be safe for concurrent
// use unless they document otherwise.
type Printer interface {
	Send(j Job) error
}

// Output is the low-level write contract implemented by Jobs.
//
// Output is aliased from an internal utility type.
type Output = utils.Output

// Job is a single render operation emitted by Cursor.
//
// Concrete jobs such as JobHeadOpen, JobText, and JobComp let custom printers
// observe or transform the stream while still sharing one cancellation context.
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
// The default printer stops early when j.Context() is canceled or expired.
func (p *printer) Send(j Job) error {
	if j.Context().Err() != nil {
		return j.Context().Err()
	}
	return j.Output(p.w)
}

// NewPrinter returns the default Printer that writes jobs to w in order.
func NewPrinter(w io.Writer) Printer {
	return &printer{w: w}
}
