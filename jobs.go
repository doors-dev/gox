package gox

import (
	"context"
	"fmt"
	"io"

	"github.com/doors-dev/gox/internal/utils"
)

// Releaser is implemented by pooled Job types that can return themselves to an
// internal pool.
//
// Implementations are expected to be single-use: once released, the object must
// not be accessed again.
type Releaser interface {
	release()
}

// Release returns r back to its pool.
//
// Release is primarily useful when a Job is created but not sent/rendered.
// Most Job implementations in this package call release automatically from
// Output via a deferred call.
func Release(r Releaser) {
	r.release()
}

// OutputError is returned when a Job cannot be rendered due to invalid state
// (for example, missing tag name, or attempting to close a void element).
type OutputError string

func (e OutputError) Error() string { return string(e) }

var headOpenPool = utils.NewStructPool[JobHeadOpen]()

// NewJobHeadOpen constructs a JobHeadOpen.
//
// The returned job is pooled and must be treated as single-use. Typical usage
// is to send the job to a Printer; the job will release itself after Output.
func NewJobHeadOpen(ctx context.Context, id uint64, kind HeadKind, tag string, attrs Attrs) *JobHeadOpen {
	job := headOpenPool.Get()
	job.ID = id
	job.Kind = kind
	job.Tag = tag
	job.Ctx = ctx
	job.Attrs = attrs
	return job
}

// JobHeadOpen represents an "open head" job.
//
// When rendered, it emits the opening tag and attributes for a regular/void
// element. For KindContainer it produces no output.
//
// JobHeadOpen is pooled. It releases itself at the end of Output.
type JobHeadOpen struct {
	// ID is the head identifier associated with this element/container.
	// The opening and closing jobs for the same head share the same ID.
	ID uint64

	// Kind describes how this head should be rendered (regular/void/container).
	Kind HeadKind

	// Tag is the element tag name. It must be non-empty for regular/void heads.
	Tag string

	// Ctx is the context used for attribute modifiers and downstream render hooks.
	Ctx context.Context

	// Attrs is the attribute set associated with this head.
	Attrs Attrs
}

// Context returns the context associated with this job.
func (j *JobHeadOpen) Context() context.Context { return j.Ctx }

func (j *JobHeadOpen) release() {
	j.ID = 0
	j.Kind = 0
	j.Tag = ""
	j.Ctx = nil
	if j.Attrs != nil {
		j.Attrs.release()
		j.Attrs = nil
	}
	headOpenPool.Put(j)
}

// Output writes the opening tag + attributes to w.
//
// Behavior by kind:
//   - KindContainer: writes nothing and returns nil
//   - KindRegular / KindVoid: requires Tag to be non-empty and writes `<tag ...>`
func (j *JobHeadOpen) Output(w io.Writer) error {
	defer j.release()

	if j.Kind == KindContainer {
		return nil
	}
	if j.Tag == "" {
		return OutputError("void or regular element must have a name")
	}
	if err := utils.WriteTagOpenBeg(w, j.Tag); err != nil {
		return err
	}
	if err := j.Attrs.output(j.Ctx, j.Tag, w); err != nil {
		return err
	}
	return utils.WriteTagOpenEnd(w)
}

var headClosePool = utils.NewStructPool[JobHeadClose]()

// NewJobHeadClose constructs a JobHeadClose.
//
// The returned job is pooled and must be treated as single-use.
func NewJobHeadClose(ctx context.Context, id uint64, kind HeadKind, tag string) *JobHeadClose {
	job := headClosePool.Get()
	job.ID = id
	job.Kind = kind
	job.Tag = tag
	job.Ctx = ctx
	return job
}

// JobHeadClose represents a "close head" job.
//
// When rendered, it emits the closing tag for a regular element. For
// KindContainer it produces no output. Closing a void element is an error.
//
// JobHeadClose is pooled. It releases itself at the end of Output.
type JobHeadClose struct {
	// ID is the head identifier associated with this element/container.
	// The opening and closing jobs for the same head share the same ID.
	ID uint64

	// Kind describes how this head should be rendered (regular/void/container).
	Kind HeadKind

	// Tag is the element tag name. It must be non-empty for regular heads.
	Tag string

	// Ctx is the context associated with this job.
	Ctx context.Context
}

// Context returns the context associated with this job.
func (j *JobHeadClose) Context() context.Context { return j.Ctx }

func (j *JobHeadClose) release() {
	j.ID = 0
	j.Kind = 0
	j.Tag = ""
	j.Ctx = nil
	headClosePool.Put(j)
}

// Output writes the closing tag to w.
//
// Behavior by kind:
//   - KindContainer: writes nothing and returns nil
//   - KindVoid: returns an error (void elements cannot be closed)
//   - KindRegular: requires Tag to be non-empty and writes `</tag>`
func (j *JobHeadClose) Output(w io.Writer) error {
	defer j.release()

	if j.Kind == KindContainer {
		return nil
	}
	if j.Kind == KindVoid {
		return OutputError("void element cannot be closed")
	}
	if j.Kind == KindRegular && j.Tag == "" {
		return OutputError("regular element must have a name")
	}
	return utils.WriteTagClose(w, j.Tag)
}

var compPool = utils.NewStructPool[JobComp]()

// NewJobComp constructs a JobComp.
//
// The returned job is pooled and must be treated as single-use.
func NewJobComp(ctx context.Context, comp Comp) *JobComp {
	j := compPool.Get()
	j.Ctx = ctx
	j.Comp = comp
	return j
}

// JobComp renders a GoX component.
//
// Output calls Comp.Main() and, if it returns a non-nil Elem, renders it into w.
//
// JobComp is pooled. It releases itself at the end of Output.
type JobComp struct {
	Comp Comp
	Ctx  context.Context
}

// Context returns the context associated with this job.
func (j *JobComp) Context() context.Context { return j.Ctx }

func (j *JobComp) release() {
	j.Comp = nil
	j.Ctx = nil
	compPool.Put(j)
}

// Output renders the component's root element (if any) into w.
func (j *JobComp) Output(w io.Writer) error {
	defer j.release()

	if el := j.Comp.Main(); el != nil {
		return el.Render(j.Ctx, w)
	}
	return nil
}

var textPool = utils.NewStructPool[JobText]()

// NewJobText constructs a JobText.
//
// The returned job is pooled and must be treated as single-use.
func NewJobText(ctx context.Context, text string) *JobText {
	j := textPool.Get()
	j.Ctx = ctx
	j.Text = text
	return j
}

// JobText writes escaped text.
//
// JobText is pooled. It releases itself at the end of Output.
type JobText struct {
	Ctx  context.Context
	Text string
}

// Context returns the context associated with this job.
func (j *JobText) Context() context.Context { return j.Ctx }

func (j *JobText) release() {
	j.Ctx = nil
	j.Text = ""
	textPool.Put(j)
}

// Output writes Text to w with HTML escaping applied.
func (j *JobText) Output(w io.Writer) error {
	defer j.release()
	return utils.WriteEscapedText(w, j.Text)
}

var rawPool = utils.NewStructPool[JobRaw]()

// NewJobRaw constructs a JobRaw.
//
// The returned job is pooled and must be treated as single-use.
func NewJobRaw(ctx context.Context, text string) *JobRaw {
	j := rawPool.Get()
	j.Ctx = ctx
	j.Text = text
	return j
}

// JobRaw writes raw (unescaped) text.
//
// JobRaw is pooled. It releases itself at the end of Output.
type JobRaw struct {
	Ctx  context.Context
	Text string
}

// Context returns the context associated with this job.
func (j *JobRaw) Context() context.Context { return j.Ctx }

func (j *JobRaw) release() {
	j.Ctx = nil
	j.Text = ""
	rawPool.Put(j)
}

// Output writes Text to w without escaping.
func (j *JobRaw) Output(w io.Writer) error {
	defer j.release()
	return utils.WriteRawText(w, j.Text)
}

var templPool = utils.NewStructPool[JobTempl]()

// NewJobTempl constructs a JobTempl.
//
// The returned job is pooled and must be treated as single-use.
func NewJobTempl(ctx context.Context, templ Templ) *JobTempl {
	j := templPool.Get()
	j.Ctx = ctx
	j.Templ = templ
	return j
}

// JobTempl renders a templ component (github.com/a-h/templ compatible).
//
// JobTempl is pooled. It releases itself at the end of Output.
type JobTempl struct {
	Ctx   context.Context
	Templ Templ
}

// Context returns the context associated with this job.
func (j *JobTempl) Context() context.Context { return j.Ctx }

func (j *JobTempl) release() {
	j.Ctx = nil
	j.Templ = nil
	templPool.Put(j)
}

// Output renders the templ component into w.
func (j *JobTempl) Output(w io.Writer) error {
	defer j.release()
	return j.Templ.Render(j.Ctx, w)
}

var fprintPool = utils.NewStructPool[JobFprint]()

// NewJobFprint constructs a JobFprint.
//
// The returned job is pooled and must be treated as single-use.
func NewJobFprint(ctx context.Context, v any) *JobFprint {
	j := fprintPool.Get()
	j.Ctx = ctx
	j.Any = v
	return j
}

// JobFprint formats a value with fmt.Fprint, writing to an escaping writer.
//
// This is the default fallback for values that do not have specialized rendering
// behavior in Cursor.Any.
//
// JobFprint is pooled. It releases itself at the end of Output.
type JobFprint struct {
	Ctx context.Context
	Any any
}

// Context returns the context associated with this job.
func (j *JobFprint) Context() context.Context { return j.Ctx }

func (j *JobFprint) release() {
	j.Ctx = nil
	j.Any = nil
	fprintPool.Put(j)
}

// Output writes Any to w using fmt.Fprint with escaping applied.
func (j *JobFprint) Output(w io.Writer) error {
	defer j.release()

	ew := utils.NewEscapedWriter(w)
	_, err := fmt.Fprint(ew, j.Any)
	return err
}

var errorPool = utils.NewStructPool[JobError]()

// NewJobError constructs a JobError.
//
// The returned job is pooled and must be treated as single-use.
func NewJobError(ctx context.Context, err error) *JobError {
	j := errorPool.Get()
	j.Ctx = ctx
	j.Err = err
	return j
}

// JobError represents a job that fails rendering with a stored error.
//
// Output returns Err as-is.
//
// JobError is pooled. It releases itself at the end of Output.
type JobError struct {
	Ctx context.Context
	Err error
}

// Context returns the context associated with this job.
func (j *JobError) Context() context.Context { return j.Ctx }

func (j *JobError) release() {
	j.Ctx = nil
	j.Err = nil
	errorPool.Put(j)
}

// Output returns the stored error.
func (j *JobError) Output(w io.Writer) error {
	defer j.release()
	return j.Err
}

var bytesPool = utils.NewStructPool[JobBytes]()

// NewJobBytes constructs a JobBytes.
//
// The returned job is pooled and must be treated as single-use.
func NewJobBytes(ctx context.Context, b []byte) *JobBytes {
	j := bytesPool.Get()
	j.Ctx = ctx
	j.Bytes = b
	return j
}

// JobBytes writes raw bytes.
//
// JobBytes is pooled. It releases itself at the end of Output.
type JobBytes struct {
	Ctx   context.Context
	Bytes []byte
}

// Context returns the context associated with this job.
func (j *JobBytes) Context() context.Context { return j.Ctx }

func (j *JobBytes) release() {
	j.Ctx = nil
	j.Bytes = nil
	bytesPool.Put(j)
}

// Output writes Bytes directly to w.
func (j *JobBytes) Output(w io.Writer) error {
	defer j.release()

	_, err := w.Write(j.Bytes)
	return err
}
