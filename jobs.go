package gox

import (
	"context"
	"fmt"
	"io"

	"github.com/doors-dev/gox/internal/utils"
)

// Releaser returns a pooled value to its internal pool.
//
// Released values are single-use and must not be touched again.
type Releaser interface {
	release()
}

// Release returns r to its pool.
//
// Most callers never need Release because the standard Job implementations
// release themselves from Output, including when Output returns an error.
// Release is for the opposite case: a custom printer that drops a job instead
// of outputting it. Never do both: a value released twice can be handed out to
// two owners at once.
func Release(r Releaser) {
	r.release()
}

// OutputError reports invalid job state during rendering.
type OutputError string

func (e OutputError) Error() string { return string(e) }

var headOpenPool = utils.NewStructPool[JobOpen]()

// NewJobOpen returns a pooled JobOpen.
//
// The returned job is single-use and is usually sent straight to a Printer. It
// takes ownership of attrs: outputting or releasing the job also releases the
// attribute set, and Output additionally releases the Attr handles it renders,
// so the caller must not use them afterwards. attrs may be nil, in which case
// the job writes the tag with no attributes.
func NewJobOpen(ctx context.Context, id uint64, kind HeadKind, tag string, attrs Attrs) *JobOpen {
	job := headOpenPool.Get()
	job.ID = id
	job.Kind = kind
	job.Tag = tag
	job.Ctx = ctx
	job.Attrs = attrs
	return job
}

// JobOpen writes the opening half of a head.
//
// Regular and void heads emit `<tag ...>`. Container heads emit no HTML.
type JobOpen struct {
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
func (j *JobOpen) Context() context.Context { return j.Ctx }

func (j *JobOpen) release() {
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

// Output writes the opening tag and attributes to w.
//
// Behavior by kind:
//   - KindContainer: writes nothing and returns nil
//   - KindRegular / KindVoid: requires Tag to be non-empty and writes `<tag ...>`
func (j *JobOpen) Output(w io.Writer) error {
	defer j.release()

	if j.Kind == KindContainer {
		return nil
	}
	if j.Tag == "" {
		return OutputError("Regular and void elements must have a name.")
	}
	if err := utils.WriteTagOpenBeg(w, j.Tag); err != nil {
		return err
	}
	if err := j.Attrs.output(j.Ctx, j.Tag, w); err != nil {
		return err
	}
	return utils.WriteTagOpenEnd(w)
}

var headClosePool = utils.NewStructPool[JobClose]()

// NewJobClose returns a pooled JobClose.
func NewJobClose(ctx context.Context, id uint64, kind HeadKind, tag string) *JobClose {
	job := headClosePool.Get()
	job.ID = id
	job.Kind = kind
	job.Tag = tag
	job.Ctx = ctx
	return job
}

// JobClose writes the closing half of a head.
//
// Regular heads emit `</tag>`. Container heads emit no HTML. Closing a void
// head is an error.
type JobClose struct {
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
func (j *JobClose) Context() context.Context { return j.Ctx }

func (j *JobClose) release() {
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
func (j *JobClose) Output(w io.Writer) error {
	defer j.release()

	if j.Kind == KindContainer {
		return nil
	}
	if j.Kind == KindVoid {
		return OutputError("Void elements cannot be closed.")
	}
	if j.Kind == KindRegular && j.Tag == "" {
		return OutputError("Regular elements must have a name.")
	}
	return utils.WriteTagClose(w, j.Tag)
}

var textPool = utils.NewStructPool[JobText]()

// NewJobText returns a pooled JobText.
func NewJobText(ctx context.Context, text string) *JobText {
	j := textPool.Get()
	j.Ctx = ctx
	j.Text = text
	return j
}

// JobText writes escaped text.
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

// NewJobRaw returns a pooled JobRaw.
func NewJobRaw(ctx context.Context, text string) *JobRaw {
	j := rawPool.Get()
	j.Ctx = ctx
	j.Text = text
	return j
}

// JobRaw writes unescaped text.
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

// NewJobTempl returns a pooled JobTempl.
func NewJobTempl(ctx context.Context, templ Templ) *JobTempl {
	j := templPool.Get()
	j.Ctx = ctx
	j.Templ = templ
	return j
}

// JobTempl renders a templ-compatible value.
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
	if j.Templ == nil {
		return nil
	}
	return j.Templ.Render(j.Ctx, w)
}

var fprintPool = utils.NewStructPool[JobFprint]()

// NewJobFprint returns a pooled JobFprint.
func NewJobFprint(ctx context.Context, v any) *JobFprint {
	j := fprintPool.Get()
	j.Ctx = ctx
	j.Any = v
	return j
}

// JobFprint formats a value with fmt.Fprint and GoX escaping.
//
// It is the default fallback for values that do not have specialized handling
// in Cursor.Any.
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

// NewJobError returns a pooled JobError.
func NewJobError(ctx context.Context, err error) *JobError {
	j := errorPool.Get()
	j.Ctx = ctx
	j.Err = err
	return j
}

// JobError fails rendering with a stored error.
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

// NewJobBytes returns a pooled JobBytes.
func NewJobBytes(ctx context.Context, b []byte) *JobBytes {
	j := bytesPool.Get()
	j.Ctx = ctx
	j.Bytes = b
	return j
}

// JobBytes writes bytes without escaping.
//
// It is the []byte counterpart of JobRaw, so the caller is responsible for the
// safety of the content. NewJobBytes keeps the caller's slice instead of
// copying it: the slice must stay unmodified until the job is output, which a
// Printer that buffers jobs may defer well past Send.
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
