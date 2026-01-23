package gox

import (
	"context"
	"fmt"
	"io"

	"github.com/doors-dev/gox/utils"
)

type Releaser interface {
	release()
}

func Release(r Releaser) {
	r.release()
}

type Error string

func (e Error) Error() string { return string(e) }

var headOpenPool = utils.NewStructPool[JobHeadOpen]()

func NewJobHeadOpen(id uint64, kind HeadKind, tag string, ctx context.Context, attrs Attrs) *JobHeadOpen {
	job := headOpenPool.Get()
	job.ID = id
	job.Kind = kind
	job.Tag = tag
	job.Ctx = ctx
	job.Attrs = attrs
	return job
}

type JobHeadOpen struct {
	ID    uint64
	Kind  HeadKind
	Tag   string
	Ctx   context.Context
	Attrs Attrs
}

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

func (j *JobHeadOpen) Output(w io.Writer) error {
	defer j.release()
	if j.Kind == KindContainer {
		return nil
	}
	if j.Tag == "" {
		return Error("void or regular element must have a name")
	}
	if err := utils.WriteTagOpenBeg(w, j.Tag); err != nil {
		return err
	}
	if err := j.Attrs.output(w); err != nil {
		return err
	}
	return utils.WriteTagOpenEnd(w)
}

var headClosePool = utils.NewStructPool[JobHeadClose]()

func NewJobHeadClose(id uint64, kind HeadKind, tag string, ctx context.Context) *JobHeadClose {
	job := headClosePool.Get()
	job.ID = id
	job.Kind = kind
	job.Tag = tag
	job.Ctx = ctx
	return job
}

type JobHeadClose struct {
	ID   uint64
	Kind HeadKind
	Tag  string
	Ctx  context.Context
}

func (j *JobHeadClose) Context() context.Context { return j.Ctx }

func (j *JobHeadClose) release() {
	j.ID = 0
	j.Kind = 0
	j.Tag = ""
	j.Ctx = nil
	headClosePool.Put(j)
}

func (j *JobHeadClose) Output(w io.Writer) error {
	defer j.release()
	if j.Kind == KindContainer {
		return nil
	}
	if j.Kind == KindVoid {
		return Error("void element cannot be closed")
	}
	if j.Kind == KindRegular && j.Tag == "" {
		return Error("regular element must have a name")
	}
	return utils.WriteTagClose(w, j.Tag)
}

var compPool = utils.NewStructPool[JobComp]()

func NewJobComp(ctx context.Context, comp Comp) *JobComp {
	j := compPool.Get()
	j.Ctx = ctx
	j.Comp = comp
	return j
}

type JobComp struct {
	Comp Comp
	Ctx  context.Context
}

func (j *JobComp) Context() context.Context { return j.Ctx }

func (j *JobComp) release() {
	j.Comp = nil
	j.Ctx = nil
	compPool.Put(j)
}

func (j *JobComp) Output(w io.Writer) error {
	defer j.release()
	return j.Comp.Main().Render(j.Ctx, w)
}

var textPool = utils.NewStructPool[JobText]()

func NewJobText(ctx context.Context, text string) *JobText {
	j := textPool.Get()
	j.Ctx = ctx
	j.Text = text
	return j
}

type JobText struct {
	Ctx  context.Context
	Text string
}

func (j *JobText) Context() context.Context { return j.Ctx }

func (j *JobText) release() {
	j.Ctx = nil
	j.Text = ""
	textPool.Put(j)
}

func (j *JobText) Output(w io.Writer) error {
	defer j.release()
	return utils.WriteEscapedText(w, j.Text)
}

var rawPool = utils.NewStructPool[JobRaw]()

func NewJobRaw(ctx context.Context, text string) *JobRaw {
	j := rawPool.Get()
	j.Ctx = ctx
	j.Text = text
	return j
}

type JobRaw struct {
	Ctx  context.Context
	Text string
}

func (j *JobRaw) Context() context.Context { return j.Ctx }

func (j *JobRaw) release() {
	j.Ctx = nil
	j.Text = ""
	rawPool.Put(j)
}

func (j *JobRaw) Output(w io.Writer) error {
	defer j.release()
	return utils.WriteRawText(w, j.Text)
}

var funcPool = utils.NewStructPool[JobFunc]()

func NewJobFunc(ctx context.Context, fn func(w io.Writer) error) *JobFunc {
	j := funcPool.Get()
	j.Ctx = ctx
	j.Func = fn
	return j
}

type JobFunc struct {
	Ctx  context.Context
	Func func(w io.Writer) error
}

func (j *JobFunc) Context() context.Context { return j.Ctx }

func (j *JobFunc) release() {
	j.Ctx = nil
	j.Func = nil
	funcPool.Put(j)
}

func (j *JobFunc) Output(w io.Writer) error {
	defer j.release()
	return j.Func(w)
}

var templPool = utils.NewStructPool[JobTempl]()

func NewJobTempl(ctx context.Context, templ Templ) *JobTempl {
	j := templPool.Get()
	j.Ctx = ctx
	j.Templ = templ
	return j
}

type JobTempl struct {
	Ctx   context.Context
	Templ Templ
}

func (j *JobTempl) Context() context.Context { return j.Ctx }

func (j *JobTempl) release() {
	j.Ctx = nil
	j.Templ = nil
	templPool.Put(j)
}

func (j *JobTempl) Output(w io.Writer) error {
	defer j.release()
	return j.Templ.Render(j.Ctx, w)
}

var fprintPool = utils.NewStructPool[JobFprint]()

func NewJobFprint(ctx context.Context, v any) *JobFprint {
	j := fprintPool.Get()
	j.Ctx = ctx
	j.Any = v
	return j
}

type JobFprint struct {
	Ctx context.Context
	Any any
}

func (j *JobFprint) Context() context.Context { return j.Ctx }

func (j *JobFprint) release() {
	j.Ctx = nil
	j.Any = nil
	fprintPool.Put(j)
}

func (j *JobFprint) Output(w io.Writer) error {
	defer j.release()
	ew := &utils.EscapedWriter{W: w}
	_, err := fmt.Fprint(ew, j.Any)
	return err
}


var errorPool = utils.NewStructPool[JobError]()

func NewJobError(ctx context.Context, err error) *JobError {
	j := errorPool.Get()
	j.Ctx = ctx
	j.Err = err
	return j
}

type JobError struct {
	Ctx context.Context
	Err error
}

func (j *JobError) Context() context.Context { return j.Ctx }

func (j *JobError) release() {
	j.Ctx = nil
	j.Err = nil
	errorPool.Put(j)
}

func (j *JobError) Output(w io.Writer) error {
	defer j.release()
	return j.Err
}
 
var bytesPool = utils.NewStructPool[JobBytes]()

func NewJobBytes(ctx context.Context, b []byte) *JobBytes {
	j := bytesPool.Get()
	j.Ctx = ctx
	j.Data = b
	return j
}

type JobBytes struct {
	Ctx context.Context
	Data   []byte
}

func (j *JobBytes) Context() context.Context { return j.Ctx }

func (j *JobBytes) release() {
	j.Ctx = nil
	j.Data = nil
	bytesPool.Put(j)
}

func (j *JobBytes) Output(w io.Writer) error {
	defer j.release()
	_, err := w.Write(j.Data)
	return err
}

