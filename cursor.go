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
	KindRegular HeadKind = iota
	KindVoid
)

func (k HeadKind) IsVoid() bool {
	return k == KindVoid
}

type head struct {
	id   uint64
	kind HeadKind
	tag  string
}

func (h head) isValid() bool {
	return h.id != 0
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
	if !last.isValid() {
		return nil
	}
	if !s.initiated() && last.kind != KindVoid {
		return nil
	}
	return HeadError("head is not in the opened state")
}

func (s *stack) Submit(p *proxyManager, ctx context.Context) error {
	if !s.initiated() {
		return errors.New("nothing to submit")
	}
	last := s.last()
	err := p.Send(NewJobHeadOpen(last.id, last.kind, last.tag, ctx, s.attrs))
	if last.kind == KindVoid {
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
		kind: KindRegular,
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
		kind: KindVoid,
		id:   s.headId(),
		tag:  name,
	})
	return nil
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

func NewCursor(ctx context.Context, printer Printer) Cursor {
	return &cursor{
		printer: newProxyManager(printer),
		stack:   stack{id: stackId.Add(1)},
		ctx:     ctx,
	}
}

type cursor struct {
	stack   stack
	printer *proxyManager
	ctx     context.Context
}

func (c Cursor) Noop(any) {}

func (c Cursor) Proxy(proxy ...Proxy) {
	for _, p := range proxy {
		c.printer.Add(c.ctx, p)
	}
}

func (c Cursor) Init(tag string) error {
	return c.stack.Init(c.ctx, tag)
}

func (c Cursor) InitVoid(tag string) error {
	return c.stack.InitVoid(c.ctx, tag)
}

func (c Cursor) Submit() error {
	return c.stack.Submit(c.printer, c.ctx)
}

func (c Cursor) Close() error {
	return c.stack.Close(c.printer, c.ctx)
}

func (c Cursor) Comp(ctx context.Context, comp Comp) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(newJobComp(ctx, comp))
}

func (c Cursor) Text(text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(newJobText(c.ctx, text))
}

func (c Cursor) Raw(text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobRaw(c.ctx, text))
}

func (c Cursor) Func(f func(w io.Writer) error) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(newJobFunc(c.ctx, f))
}

func (c Cursor) Templ(templ Templ) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(newJobTempl(c.ctx, templ))
}

func (c Cursor) Fprint(any any) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(newJobFprint(c.ctx, any))
}

func (c *cursor) terminate() {
	c.printer.terminate()
}

func (c Cursor) Job(job Job) error {
	return c.printer.Send(job)
}

func (c Cursor) Provider(provider Provider) error {
	return c.Job(provider.Job(c.ctx))
}

func (c Cursor) Any(ctx context.Context, any any) error {
	if any == nil {
		return nil
	}
	switch v := any.(type) {
	case string:
		return c.Text(v)
	case []string:
		for _, v := range v {
			if err := c.Text(v); err != nil {
				return err
			}
		}
		return nil
	case Elem:
		return c.Comp(ctx, v)
	case []Elem:
		for _, v := range v {
			if err := c.Comp(ctx, v); err != nil {
				return err
			}
		}
		return nil
	case Comp:
		return c.Comp(ctx, v)
	case []Comp:
		for _, v := range v {
			if err := c.Comp(ctx, v); err != nil {
				return err
			}
		}
		return nil
	case func(w io.Writer) error:
		return c.Func(v)
	case Job:
		return c.Job(v)
	case []Job:
		for _, v := range v {
			if err := c.Job(v); err != nil {
				return err
			}
		}
		return nil
	case Provider:
		return c.Provider(v)
	case Templ:
		return c.Templ(v)
	case []interface{}:
		for _, v := range v {
			if err := c.Any(ctx, v); err != nil {
				return err
			}
		}
		return nil
	default:
		return c.Fprint(any)
	}
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

func (c Cursor) AttrMod(mut ...AttrMod) error {
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
