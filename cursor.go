package gox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
)

type Context = context.Context

type HeadError string

func (e HeadError) Error() string {
	return string(e)
}

type HeadKind int

const (
	Container HeadKind = iota
	Regular
	Void
)

type head struct {
	id   uint64
	kind HeadKind
	tag  string
}

var stackId = atomic.Uint32{}

type stack struct {
	heads   []head
	attrs   *attrs
	ctx     context.Context
	id      uint32
	counter uint32
}

func (s *stack) headId() uint64 {
	s.counter++
	return uint64(s.id)<<32 | uint64(s.counter)
}

func (s *stack) initiated() bool {
	return s.ctx != nil
}

func (s *stack) last() head {
	if len(s.heads) == 0 {
		return head{}
	}
	return s.heads[len(s.heads)-1]
}

func (s *stack) Opened() error {
	last := s.last()
	if last.id == 0 {
		return nil
	}
	if !s.initiated() && last.kind != Void {
		return nil
	}
	return HeadError("head is not in the opened state")
}

func (s *stack) Submit(p *proxyManager, ctx context.Context) error {
	if s.ctx == nil {
		return errors.New("nothing to submit")
	}
	last := s.last()
	err := p.Send(NewJobHeadOpen(last.id, last.kind, last.tag, ctx, s.attrs))
	if last.kind == Void {
		s.heads = s.heads[:len(s.heads)-1]
	}
	s.ctx = nil
	s.attrs = nil
	return err
}

func (s *stack) Close(p *proxyManager, ctx context.Context) error {
	last := s.last()
	if last.id == 0 {
		return errors.New("nothing to close")
	}
	s.heads = s.heads[:len(s.heads)-1]
	return p.Send(NewJobHeadClose(last.id, last.kind, last.tag, ctx))
}

func (s *stack) Init(ctx context.Context, name string) error {
	if err := s.Opened(); err != nil {
		return err
	}
	s.ctx = ctx
	s.heads = append(s.heads, head{
		kind: Regular,
		id:   s.headId(),
		tag:  name,
	})
	return nil
}

func (s *stack) InitVoid(ctx context.Context, name string) error {
	if err := s.Opened(); err != nil {
		return err
	}
	s.ctx = ctx
	s.heads = append(s.heads, head{
		kind: Void,
		id:   s.headId(),
		tag:  name,
	})
	return nil
}

func (s *stack) InitContainer(p *proxyManager, ctx context.Context) error {
	if err := s.Opened(); err != nil {
		return err
	}
	head := head{
		kind: Void,
	}
	s.heads = append(s.heads, head)
	return p.Send(NewJobHeadOpen(head.id, head.kind, head.tag, s.ctx, nil))
}

func (s *stack) Attrs() (*attrs, error) {
	if !s.initiated() {
		return nil, HeadError("head is not in the init state")
	}
	if s.attrs == nil {
		s.attrs = NewAttrs(s.ctx)
	}
	return s.attrs, nil
}

type Cursor = *cursor

func NewCursor(printer Printer) Cursor {
	return &cursor{
		printer: newProxyManager(printer),
		stack:   stack{id: stackId.Add(1)},
	}
}

type cursor struct {
	stack   stack
	printer *proxyManager
}

func (c Cursor) AddProxy(proxy ...Proxy) {
	for _, p := range proxy {
		c.printer.Add(p)
	}
}

func (c Cursor) HeadInit(ctx context.Context, tag string) error {
	return c.stack.Init(ctx, tag)
}

func (c Cursor) HeadInitVoid(ctx context.Context, tag string) error {
	return c.stack.InitVoid(ctx, tag)
}

func (c Cursor) HeadInitCont(ctx context.Context) error {
	return c.stack.InitContainer(c.printer, ctx)
}

func (c Cursor) HeadSubmit(ctx context.Context) error {
	return c.stack.Submit(c.printer, ctx)
}

func (c Cursor) HeadClose(ctx context.Context) error {
	return c.stack.Close(c.printer, ctx)
}

func (c Cursor) WriteElem(ctx context.Context, elem Elem) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(JobElem{
		Elem: elem,
		Ctx:  ctx,
	})
}

func (c Cursor) WriteComp(ctx context.Context, comp Comp) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	if elem, ok := comp.(Elem); ok {
		return c.WriteElem(ctx, elem)
	}
	return c.printer.Send(JobComp{
		Comp: comp,
		Ctx:  ctx,
	})
}

func (c Cursor) WriteText(ctx context.Context, text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(newJobText(ctx, text))
}

func (c Cursor) WriteRaw(ctx context.Context, text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(JobRaw{
		Ctx:  ctx,
		Text: text,
	})
}

func (c Cursor) WriteFunc(ctx context.Context, f func(w io.Writer) error) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(JobFunc{
		Ctx:  ctx,
		Func: f,
	})
}

func (c Cursor) WriteTempl(ctx context.Context, templ Templ) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(JobTempl{
		Ctx:   ctx,
		Templ: templ,
	})
}

func (c Cursor) WriteFprint(ctx context.Context, any any) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(JobFprint{
		Ctx: ctx,
		Any: any,
	})
}

func (c Cursor) WriteJob(ctx context.Context, job Job) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(job)
}

func (c Cursor) WriteAny(ctx context.Context, any any) error {
	if any == nil {
		return nil
	}
	switch v := any.(type) {
	case string:
		return c.WriteText(ctx, v)
	case Elem:
		return c.WriteElem(ctx, v)
	case Comp:
		return c.WriteComp(ctx, v)
	case func(w io.Writer) error:
		return c.WriteFunc(ctx, v)
	case Job:
		return c.WriteJob(ctx, v)
	case Templ:
		return c.WriteTempl(ctx, v)
	default:
		return c.WriteFprint(ctx, any)
	}
}

func (c Cursor) WriteAnyFunc(ctx context.Context, f func() any) error {
	v := f()
	return c.WriteAny(ctx, v)
}

func (c Cursor) AttrSetAny(name string, value any) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	attr := attrs.Get(name)
	switch v := value.(type) {
	case bool:
		attr.SetBool(v)
	case string:
		attr.Set(v)
	default:
		attr.Set(fmt.Sprint(value))
	}
	return nil
}

func (c Cursor) AttrSetBool(name string, value bool) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	attr := attrs.Get(name)
	attr.SetBool(value)
	return nil
}

func (c Cursor) AttrSet(name string, value string) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	attr := attrs.Get(name)
	attr.Set(value)
	return nil
}

func (c Cursor) AttrAppend(name string, value string) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	attr := attrs.Get(name)
	attr.Append(value)
	return nil
}

func (c Cursor) AttrAppendObject(name string, value any) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	attr := attrs.Get(name)
	attr.AppendObject(value)
	return nil
}

func (c Cursor) AttrSetObject(name string, value any) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	attr := attrs.Get(name)
	attr.SetObject(value)
	return nil
}


func (c Cursor) AttrMut(mut ...AttrMut) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	for _, m := range mut {
		if err := attrs.Mutate(m); err != nil {
			return err
		}
	}
	return nil
}
