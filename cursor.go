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
	KindContainer HeadKind = iota
	KindRegular
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

var stackID = atomic.Uint32{}

type stack struct {
	heads     []head
	attrs     *attrs
	ctx       context.Context
	submitted bool
	id        uint32
	counter   uint32
}

func (s *stack) headID() uint64 {
	s.counter++
	return uint64(s.id)<<32 | uint64(s.counter)
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
	if s.submitted && last.kind != KindVoid {
		return nil
	}
	return HeadError("head is not in the opened state")
}

func (s *stack) Submit(p Printer) error {
	if s.submitted {
		return errors.New("head is already submitted")
	}
	last := s.last()
	if last.kind == KindVoid {
		s.heads = s.heads[:len(s.heads)-1]
	}
	err := p.Send(NewJobHeadOpen(last.id, last.kind, last.tag, s.ctx, s.attrs))
	s.submitted = true
	s.attrs = nil
	return err
}

func (s *stack) Close(p Printer) error {
	if !s.submitted {
		return errors.New("head neads to be submitted before closing")
	}
	last := s.last()
	if !last.isValid() {
		return errors.New("nothing to close")
	}
	s.submitted = true
	s.heads = s.heads[:len(s.heads)-1]
	return p.Send(NewJobHeadClose(last.id, last.kind, last.tag, s.ctx))
}

func (s *stack) Init(name string) error {
	if err := s.Opened(); err != nil {
		return err
	}
	s.submitted = false
	s.heads = append(s.heads, head{
		kind: KindRegular,
		id:   s.headID(),
		tag:  name,
	})
	s.attrs = NewAttrs(s.ctx)
	return nil
}

func (s *stack) InitVoid(name string) error {
	if err := s.Opened(); err != nil {
		return err
	}
	s.submitted = false
	s.heads = append(s.heads, head{
		kind: KindVoid,
		id:   s.headID(),
		tag:  name,
	})
	s.attrs = NewAttrs(s.ctx)
	return nil
}

func (s *stack) InitSubmitContainer(p Printer) error {
	if err := s.Opened(); err != nil {
		return err
	}
	s.submitted = false
	s.heads = append(s.heads, head{
		kind: KindContainer,
		id:   s.headID(),
	})
	return s.Submit(p)
}

func (s *stack) Attrs() (*attrs, error) {
	if s.submitted {
		return nil, HeadError("head is already submitted")
	}
	return s.attrs, nil
}

type Cursor = *cursor

func NewCursor(ctx context.Context, printer Printer) Cursor {
	return &cursor{
		printer: printer,
		stack:   stack{id: stackID.Add(1), submitted: true, ctx: ctx},
		ctx:     ctx,
	}
}

type cursor struct {
	stack   stack
	printer Printer
	ctx     context.Context
	proxies []Proxy
}

func (c Cursor) Context() context.Context {
	return c.ctx
}

func (c Cursor) NewID() uint64 {
	return c.stack.headID()
}

func (c Cursor) Noop(any) {}

func (c Cursor) AddProxy(proxies ...Proxy) {
	c.proxies = append(c.proxies, proxies...)
}

func (c Cursor) ProxyElem(elem Elem) error {
	for i := len(c.proxies) - 1; i > 0; i-- {
		proxy := c.proxies[i]
		c.proxies[i] = nil
		elem = Elem(func(cur Cursor) error {
			return proxy.Proxy(cur, elem)
		})
	}
	c.proxies = c.proxies[:0]
	return elem(c)
}

func (c Cursor) Init(tag string) error {
	return c.stack.Init(tag)
}

func (c Cursor) InitVoid(tag string) error {
	return c.stack.InitVoid(tag)
}

func (c Cursor) InitContainer() error {
	return c.stack.InitSubmitContainer(c.printer)
}

func (c Cursor) Submit() error {
	return c.stack.Submit(c.printer)
}

func (c Cursor) Close() error {
	return c.stack.Close(c.printer)
}

func (c Cursor) Comp(comp Comp) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobComp(c.ctx, comp))
}

func (c Cursor) CompCtx(ctx context.Context, comp Comp) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobComp(ctx, comp))
}

func (c Cursor) Text(text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobText(c.ctx, text))
}

func (c Cursor) Raw(text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobRaw(c.ctx, text))
}

func (c Cursor) Bytes(data []byte) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobBytes(c.ctx, data))
}

func (c Cursor) Func(f func(w io.Writer) error) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobFunc(c.ctx, f))
}

func (c Cursor) Templ(templ Templ) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobTempl(c.ctx, templ))
}

func (c Cursor) TemplCtx(ctx context.Context, templ Templ) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobTempl(ctx, templ))
}

func (c Cursor) Fprint(any any) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobFprint(c.ctx, any))
}

func (c Cursor) Send(job Job) error {
	return c.printer.Send(job)
}

func (c Cursor) Editor(editor Editor) error {
	return editor.Edit(c)
}

func (c Cursor) Many(many ...any) error {
	for _, any := range many {
		if err := c.Any(any); err != nil {
			return err
		}
	}
	return nil
}

func (c Cursor) Any(any any) error {
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
		return c.Comp(v)
	case []Elem:
		for _, v := range v {
			if err := c.Comp(v); err != nil {
				return err
			}
		}
		return nil
	case Comp:
		return c.Comp(v)
	case []Comp:
		for _, v := range v {
			if err := c.Comp(v); err != nil {
				return err
			}
		}
		return nil
	case func(w io.Writer) error:
		return c.Func(v)
	case Job:
		return c.Send(v)
	case []Job:
		for _, v := range v {
			if err := c.Send(v); err != nil {
				return err
			}
		}
		return nil
	case Editor:
		return c.Editor(v)
	case Templ:
		return c.Templ(v)
	case []interface{}:
		return c.Many(v...)
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
	if value == nil {
		attr.SetBool(true)
		return nil
	}
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

func (c Cursor) AttrMod(mods ...AttrMod) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	for _, m := range mods {
		attrs.AddMod(m)
	}
	return nil
}
