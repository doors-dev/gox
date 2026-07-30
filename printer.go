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

// Output is the low-level write contract:
//
//	Output(w io.Writer) error
//
// Jobs implement it to write themselves to the underlying writer. It is also
// the per-value attribute hook: an attribute value that implements Output is
// serialized by calling its Output with an escaping writer instead of being
// formatted with fmt.Fprint, so the value chooses its own bytes but still
// cannot emit raw markup.
type Output = utils.Output

// Job is a single render operation emitted by Cursor.
//
// Concrete jobs such as JobHeadOpen, JobText, and JobComp let custom printers
// observe or transform the stream. Each job carries its own context: the
// cursor's context by default, or the one passed to Cursor.CompCtx,
// Cursor.TemplCtx, or a NewJob* constructor, so the jobs of one stream do not
// necessarily share a cancellation signal.
//
// The jobs GoX emits are pooled and single-use: Output returns the job to its
// pool and clears its fields, so a job may be output at most once and must not
// be inspected or resent afterwards.
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
//
// Send checks the job's context first: when it is already canceled or expired,
// the job is skipped, nothing is written, and Send returns that context's
// error, which surfaces as the error from Elem.Render or the enclosing Elem.
func NewPrinter(w io.Writer) Printer {
	return &printer{w: w}
}
