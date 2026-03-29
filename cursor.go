package gox

import (
	"context"
	"errors"
	"sync/atomic"
)

// HeadError reports an invalid Cursor state transition.
//
// Examples include emitting child content before Submit or mutating attributes
// after a head has already been submitted.
type HeadError string

func (e HeadError) Error() string { return string(e) }

// HeadKind identifies what kind of head Cursor is building.
//
// The kind controls whether the head emits a real tag and whether it may have
// children.
type HeadKind int

const (
	// KindContainer is a synthetic head used to group a sequence of jobs without
	// emitting an actual HTML tag. It is submitted immediately.
	KindContainer HeadKind = iota

	// KindRegular is a normal, non-void HTML element.
	KindRegular

	// KindVoid is a void/self-closing HTML element (e.g. <input>, <br>, etc.).
	// Void heads are submitted as an open job and then removed from the stack;
	// they never accept children and must not be closed.
	KindVoid
)

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
	return HeadError("The head is not open.")
}

func (s *stack) Submit(p Printer) error {
	if s.submitted {
		return errors.New("The head has already been submitted.")
	}
	last := s.last()
	if last.kind == KindVoid {
		s.heads = s.heads[:len(s.heads)-1]
	}
	err := p.Send(NewJobHeadOpen(s.ctx, last.id, last.kind, last.tag, s.attrs))
	s.submitted = true
	s.attrs = nil
	return err
}

func (s *stack) Close(p Printer) error {
	if !s.submitted {
		return errors.New("Submit the head before closing it.")
	}
	last := s.last()
	if !last.isValid() {
		return errors.New("There is nothing to close.")
	}
	s.submitted = true
	s.heads = s.heads[:len(s.heads)-1]
	return p.Send(NewJobHeadClose(s.ctx, last.id, last.kind, last.tag))
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
	s.attrs = NewAttrs()
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
	s.attrs = NewAttrs()
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
		return nil, HeadError("The head has already been submitted.")
	}
	return s.attrs, nil
}

// Cursor builds output by streaming Jobs to a Printer.
//
// Most `.gox` users never construct a Cursor directly because generated code
// does it for them. Reach for Cursor when you need manual rendering, custom
// editors, or proxy/printer integrations.
//
// Example:
//
//	cur := gox.NewCursor(ctx, gox.NewPrinter(w))
//	_ = cur.Init("span")
//	_ = cur.AttrSet("class", "badge")
//	_ = cur.Submit()
//	_ = cur.Text("New")
//	_ = cur.Close()
//
// Cursor maintains a stack of active heads to validate nesting and enforce a
// small state machine:
//
// Regular element lifecycle:
//  1. Init(tag)
//  2. (optional) AttrSet / AttrMod
//  3. Submit()              // emits head-open job
//  4. emit children jobs    // Text/Comp/Any/etc.
//  5. Close()               // emits head-close job
//
// Void element lifecycle:
//  1. InitVoid(tag)
//  2. (optional) AttrSet / AttrMod
//  3. Submit()              // emits head-open job; no children and no Close
//
// Container lifecycle:
//  1. InitContainer()       // emits container head-open job immediately
//  2. emit children jobs
//  3. Close()               // emits container head-close job
//
// # Content state
//
// Several methods require the cursor to be in a content state, meaning:
//
//   - no element head is active (top-level), OR
//   - the current element/container head has already been submitted with Submit,
//     and may accept children.
//
// Cursor is not safe for concurrent use.
type Cursor = *cursor

// NewCursor returns a Cursor that emits jobs to printer.
//
// ctx becomes the default context for jobs created through this cursor. The
// returned cursor starts at top level, so callers may emit content immediately
// or begin a new head with Init, InitVoid, or InitContainer.
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
}

// Context returns the default context for jobs emitted by this cursor.
func (c Cursor) Context() context.Context {
	return c.ctx
}

// NewID returns a process-unique id for correlating render-time state.
//
// IDs increase monotonically within one cursor.
func (c Cursor) NewID() uint64 {
	return c.stack.headID()
}

// Init starts a regular element head.
//
// After Init, callers may set attributes with AttrSet or AttrMod. Child content
// must wait until Submit succeeds.
func (c Cursor) Init(tag string) error {
	return c.stack.Init(tag)
}

// InitVoid starts a void element head.
//
// Void heads may receive attributes before Submit, but they never accept
// children and must not be closed.
func (c Cursor) InitVoid(tag string) error {
	return c.stack.InitVoid(tag)
}

// InitContainer starts a synthetic container head and submits it immediately.
//
// Containers do not emit an HTML tag. They group a range of child jobs under a
// shared head id and must still be closed with Close.
func (c Cursor) InitContainer() error {
	return c.stack.InitSubmitContainer(c.printer)
}

// Submit emits the current head's opening job.
//
// After Submit, the current head is open for child content and no longer
// accepts attribute mutation.
func (c Cursor) Submit() error {
	return c.stack.Submit(c.printer)
}

// Close closes the current regular or container head.
//
// The current head must already be submitted. Void elements must not be closed.
func (c Cursor) Close() error {
	return c.stack.Close(c.printer)
}

// Comp emits comp at the current cursor position.
func (c Cursor) Comp(comp Comp) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobComp(c.ctx, comp))
}

// CompCtx is like Comp but uses ctx for the emitted job.
func (c Cursor) CompCtx(ctx context.Context, comp Comp) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobComp(ctx, comp))
}

// Text emits escaped text at the current cursor position.
func (c Cursor) Text(text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobText(c.ctx, text))
}

// Raw emits unescaped text at the current cursor position.
func (c Cursor) Raw(text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobRaw(c.ctx, text))
}

// Bytes emits raw bytes at the current cursor position.
func (c Cursor) Bytes(data []byte) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobBytes(c.ctx, data))
}

// Templ emits a templ-compatible component at the current cursor position.
func (c Cursor) Templ(templ Templ) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobTempl(c.ctx, templ))
}

// TemplCtx is like Templ but uses ctx for the emitted job.
func (c Cursor) TemplCtx(ctx context.Context, templ Templ) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobTempl(ctx, templ))
}

// Fprint renders any with fmt.Fprint and GoX escaping.
func (c Cursor) Fprint(any any) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobFprint(c.ctx, any))
}

// Send forwards job directly to the underlying Printer.
//
// Send skips cursor state validation, so callers must preserve any ordering and
// nesting guarantees they need.
func (c Cursor) Send(job Job) error {
	return c.printer.Send(job)
}

// Editor applies editor to this cursor.
func (c Cursor) Editor(editor Editor) error {
	return editor.Edit(c)
}

// Many renders each value in order using Any.
//
// Many is a convenient way to emit mixed values without switching manually.
func (c Cursor) Many(many ...any) error {
	for _, any := range many {
		if err := c.Any(any); err != nil {
			return err
		}
	}
	return nil
}

// Any renders a value using GoX's default dynamic dispatch.
//
// Supported cases include:
//   - string / []string
//   - Elem / []Elem
//   - Comp / []Comp
//   - Job / []Job
//   - Editor
//   - Templ
//   - []any (treated as a variadic list)
//
// Nil interface values are ignored. Everything else falls back to Fprint.
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
	case Editor:
		return c.Editor(v)
	case Comp:
		return c.Comp(v)
	case []Comp:
		for _, v := range v {
			if err := c.Comp(v); err != nil {
				return err
			}
		}
		return nil
	case Job:
		return c.Send(v)
	case []Job:
		for _, v := range v {
			if err := c.Send(v); err != nil {
				return err
			}
		}
		return nil
	case Templ:
		return c.Templ(v)
	case []interface{}:
		return c.Many(v...)
	default:
		return c.Fprint(any)
	}
}

// AttrSet sets an attribute on the current head.
//
// AttrSet may be used only after Init or InitVoid and before Submit.
func (c Cursor) AttrSet(name string, value any) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	attrs.Get(name).Set(value)
	return nil
}

// AttrMod adds one or more modifiers to the current head.
//
// AttrMod may be used only after Init or InitVoid and before Submit. Modifiers
// run right before rendering and may inspect, leave unchanged, or mutate the
// full attribute set.
func (c Cursor) AttrMod(mods ...Modify) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	for _, m := range mods {
		attrs.AddMod(m)
	}
	return nil
}
