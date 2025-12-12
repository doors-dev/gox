package gox

import (
	"context"
	"fmt"
	"io"

	"github.com/doors-dev/gox/utils"
)

type JobError string

func (e JobError) Error() string {
	return string(e)
}

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

func (j *JobHeadOpen) Context() context.Context {
	return j.Ctx
}

func (j *JobHeadOpen) release() {
	j.Ctx = nil
	j.Attrs = nil
	headOpenPool.Put(j)
}

func (j *JobHeadOpen) Output(w io.Writer) error {
	defer j.release()
	if j.Kind == Container {
		return nil
	}
	if j.Tag == "" {
		return JobError("void or regular element must have a name")
	}
	if err := utils.WriteTagOpenBeg(w, j.Tag); err != nil {
		return err
	}
	if err := j.Attrs.output(w); err != nil {
		return err
	}
	if err := utils.WriteTagOpenEnd(w); err != nil {
		return err
	}
	return nil
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

func (j *JobHeadClose) release() {
	j.Ctx = nil
	headClosePool.Put(j)
}

func (j *JobHeadClose) Output(w io.Writer) error {
	defer j.release()
	if j.Kind == Void {
		return JobError("void element cannot be closed")
	}
	if j.Kind == Regular && j.Tag == "" {
		return JobError("regular element must have a name")
	}
	return utils.WriteTagClose(w, j.Tag)
}

func (j *JobHeadClose) Context() context.Context {
	return j.Ctx
}

type JobElem struct {
	Elem Elem
	Ctx  context.Context
}

func (j JobElem) Context() context.Context {
	return j.Ctx
}

func (j JobElem) Output(w io.Writer) error {
	return j.Elem.Render(j.Ctx, w)
}

type JobComp struct {
	Comp Comp
	Ctx  context.Context
}

func (j JobComp) Context() context.Context {
	return j.Ctx
}

func (j JobComp) Output(w io.Writer) error {
	job := JobElem{
		Ctx:  j.Ctx,
		Elem: j.Comp.Main(),
	}
	return job.Output(w)
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

func (j *JobText) release() {
	j.Ctx = nil
	j.Text = ""
	textPool.Put(j)
}

func (j *JobText) Context() context.Context {
	return j.Ctx
}

func (j *JobText) Output(w io.Writer) error {
	defer j.release()
	return utils.WriteEscapedText(w, j.Text)
}

type JobRaw struct {
	Ctx  context.Context
	Text string
}

func (j JobRaw) Context() context.Context {
	return j.Ctx
}

func (j JobRaw) Output(w io.Writer) error {
	return utils.WriteRawText(w, j.Text)
}

type JobFunc struct {
	Ctx  context.Context
	Func func(w io.Writer) error
}

func (j JobFunc) Context() context.Context {
	return j.Ctx
}

func (j JobFunc) Output(w io.Writer) error {
	return j.Func(w)
}

type JobTempl struct {
	Ctx   context.Context
	Templ Templ
}

func (j JobTempl) Context() context.Context {
	return j.Ctx
}

func (j JobTempl) Output(w io.Writer) error {
	return j.Templ.Render(j.Ctx, w)
}

type JobFprint struct {
	Ctx context.Context
	Any any
}

func (j JobFprint) Context() context.Context {
	return j.Ctx
}

func (j JobFprint) Output(w io.Writer) error {
	ew := &utils.EscapedWriter{
		W: w,
	}
	_, err := fmt.Fprint(ew, j.Any)
	return err
}
