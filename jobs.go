package gox

import (
	"context"
	"fmt"
	"io"

	"github.com/doors-dev/gox/utils"
)

type JobError string

func (e JobError) Error() string { return string(e) }

var headOpenPool = utils.NewStructPool[JobHeadOpen]()

func NewJobHeadOpen(id uint64, kind HeadKind, tag string, ctx context.Context, attrs Attrs) *JobHeadOpen {
	job := headOpenPool.Get()
	job.Id = id
	job.Kind = kind
	job.Tag = tag
	job.Ctx = ctx
	job.Attrs = attrs
	return job
}

type JobHeadOpen struct {
	Id    uint64
	Kind  HeadKind
	Tag   string
	Ctx   context.Context
	Attrs Attrs
}

func (j *JobHeadOpen) Context() context.Context { return j.Ctx }

func (j *JobHeadOpen) release() {
	j.Id = 0
	j.Kind = 0
	j.Tag = ""
	j.Ctx = nil
	j.Attrs = nil
	headOpenPool.Put(j)
}

func (j *JobHeadOpen) Output(w io.Writer) error {
	defer j.release()
	if j.Tag == "" {
		return JobError("void or regular element must have a name")
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
	job.Id = id
	job.Kind = kind
	job.Tag = tag
	job.Ctx = ctx
	return job
}

type JobHeadClose struct {
	Id   uint64
	Kind HeadKind
	Tag  string
	Ctx  context.Context
}

func (j *JobHeadClose) Context() context.Context { return j.Ctx }

func (j *JobHeadClose) release() {
	j.Id = 0
	j.Kind = 0
	j.Tag = ""
	j.Ctx = nil
	headClosePool.Put(j)
}

func (j *JobHeadClose) Output(w io.Writer) error {
	defer j.release()
	if j.Kind == KindVoid {
		return JobError("void element cannot be closed")
	}
	if j.Kind == KindRegular && j.Tag == "" {
		return JobError("regular element must have a name")
	}
	return utils.WriteTagClose(w, j.Tag)
}

var elemPool = utils.NewStructPool[JobElem]()

func newJobElem(ctx context.Context, elem Elem) *JobElem {
	j := elemPool.Get()
	j.Ctx = ctx
	j.Elem = elem
	return j
}

type JobElem struct {
	Elem Elem
	Ctx  context.Context
}

func (j *JobElem) Context() context.Context { return j.Ctx }

func (j *JobElem) release() {
	j.Elem = nil
	j.Ctx = nil
	elemPool.Put(j)
}

func (j *JobElem) Output(w io.Writer) error {
	defer j.release()
	return j.Elem.Render(j.Ctx, w)
}

var compPool = utils.NewStructPool[JobComp]()

func newJobComp(ctx context.Context, comp Comp) *JobComp {
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
	ej := newJobElem(j.Ctx, j.Comp.Main())
	return ej.Output(w)
}

var textPool = utils.NewStructPool[JobText]()

func newJobText(ctx context.Context, text string) *JobText {
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

func newJobFunc(ctx context.Context, fn func(w io.Writer) error) *JobFunc {
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

func newJobTempl(ctx context.Context, templ Templ) *JobTempl {
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

func newJobFprint(ctx context.Context, v any) *JobFprint {
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
