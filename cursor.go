package gox

import (
	"context"
	"errors"
	"sync/atomic"
)

// HeadError is returned when Cursor element-head operations are performed
// in an invalid state (for example, writing node content before submitting
// the current head, or mutating attributes after submission).
//
// It is used to distinguish "render state machine" errors from other failures.
type HeadError string

func (e HeadError) Error() string { return string(e) }

// HeadKind describes the kind of an element head currently being built/rendered.
//
// The kind affects how the head is submitted and whether it can have children.
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
	err := p.Send(NewJobHeadOpen(s.ctx, last.id, last.kind, last.tag, s.attrs))
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
		return nil, HeadError("head is already submitted")
	}
	return s.attrs, nil
}

// Cursor is the low-level rendering cursor used by GoX.
//
// Cursor streams rendering operations to a Printer as a sequence of Jobs.
// It maintains a stack of active element “heads” to validate nesting and enforce
// a simple state machine:
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

// NewCursor constructs a Cursor that emits jobs to printer.
// ctx is used as the default context for jobs that accept a context.
//
// The returned cursor starts in an “opened” state at top-level: it is valid to
// emit content immediately, or to begin a new element via Init methods.
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

// Context returns the default context associated with this cursor.
func (c Cursor) Context() context.Context {
	return c.ctx
}

// NewID returns a globally unique id suitable for associating external state
// with emitted jobs.
//
// IDs are globally unique across cursors created in the same process and are
// monotonically increasing per cursor.
func (c Cursor) NewID() uint64 {
	return c.stack.headID()
}

// Init begins a new regular (non-void) element head with the given tag name.
//
// After Init, the element is in “initialization” state:
//   - attributes may be set via AttrSet/AttrMod,
//   - node content must not be emitted until Submit is called.
func (c Cursor) Init(tag string) error {
	return c.stack.Init(tag)
}

// InitVoid begins a new void (self-closing) element head with the given tag name.
//
// After InitVoid, the element is in “initialization” state (attributes may be set).
// Call Submit to emit the head-open job. Void elements cannot have children and
// must not be closed.
func (c Cursor) InitVoid(tag string) error {
	return c.stack.InitVoid(tag)
}

// InitContainer begins a synthetic container head and submits it immediately.
//
// Containers do not emit an HTML tag. They exist to group a sequence of jobs
// under a distinct head id/kind in the job stream.
//
// After InitContainer, the container head is active and must be closed with Close()
func (c Cursor) InitContainer() error {
	return c.stack.InitSubmitContainer(c.printer)
}

// Submit emits an opening head job for the current element.
//
// Submit transitions the current element from “initialization” state to “opened” state.
// After Submit succeeds:
//   - attribute mutation is no longer allowed,
//   - node-content jobs may be emitted into the element,
//   - the element must eventually be closed with Close() (except for void elements).
func (c Cursor) Submit() error {
	return c.stack.Submit(c.printer)
}

// Close emits a closing head job for the current element/container.
//
// Close requires that the current head has already been submitted (i.e., Submit
// was called successfully). Closing before submitting is an error.
//
// Void elements must not be closed.
func (c Cursor) Close() error {
	return c.stack.Close(c.printer)
}

// Comp emits a component job at the current cursor position.
//
// Comp requires the cursor to be in content state.
func (c Cursor) Comp(comp Comp) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobComp(c.ctx, comp))
}

// CompCtx is like Comp, but uses ctx for the emitted job.
//
// CompCtx requires the cursor to be in content state.
func (c Cursor) CompCtx(ctx context.Context, comp Comp) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobComp(ctx, comp))
}

// Text emits escaped text at the current cursor position.
//
// Text requires the cursor to be in content state.
func (c Cursor) Text(text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobText(c.ctx, text))
}

// Raw emits raw (unescaped) text at the current cursor position.
//
// Raw requires the cursor to be in content state.
func (c Cursor) Raw(text string) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobRaw(c.ctx, text))
}

// Bytes emits a byte-slice payload job at the current cursor position.
//
// Bytes requires the cursor to be in content state.
func (c Cursor) Bytes(data []byte) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobBytes(c.ctx, data))
}

// Templ emits a templ component job at the current cursor position.
//
// Templ requires the cursor to be in content state.
func (c Cursor) Templ(templ Templ) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobTempl(c.ctx, templ))
}

// TemplCtx is like Templ, but uses ctx for the emitted job.
//
// TemplCtx requires the cursor to be in content state.
func (c Cursor) TemplCtx(ctx context.Context, templ Templ) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobTempl(ctx, templ))
}

// Fprint emits a formatted-print job for any at the current cursor position.
//
// Fprint requires the cursor to be in content state.
func (c Cursor) Fprint(any any) error {
	if err := c.stack.Opened(); err != nil {
		return err
	}
	return c.printer.Send(NewJobFprint(c.ctx, any))
}

// Send forwards an already-constructed Job directly to the underlying Printer.
//
// Send does not perform state validation; callers are responsible for ensuring
// job ordering/nesting is valid for their use case.
func (c Cursor) Send(job Job) error {
	return c.printer.Send(job)
}

// Editor applies editor to this cursor.
//
// Editor is a hook for advanced rendering that needs direct access to cursor methods.
func (c Cursor) Editor(editor Editor) error {
	return editor.Edit(c)
}

// Many renders each value in order using Any.
//
// Many requires the cursor to be in content state fot he most types.
func (c Cursor) Many(many ...any) error {
	for _, any := range many {
		if err := c.Any(any); err != nil {
			return err
		}
	}
	return nil
}

// Any renders a value using GoX’s default dynamic dispatch.
//
// Defined types include:
//   - string / []string
//   - Elem / []Elem
//   - Comp / []Comp
//   - Job / []Job
//   - Editor
//   - Templ
//   - []interface{} (treated as a variadic list)
//
// nil values are ignored. Other types fall back to Fprint.
//
// Any requires the cursor to be in content state.
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

// AttrSet sets an attribute on the current head.
//
// AttrSet may only be used during initialization state (after Init/InitVoid and
// before Submit). After Submit, AttrSet returns an error.
func (c Cursor) AttrSet(name string, value any) error {
	attrs, err := c.stack.Attrs()
	if err != nil {
		return err
	}
	attrs.Get(name).Set(value)
	return nil
}

// AttrMod adds one or more attribute modifiers to the current head.
//
// AttrMod may only be used during initialization state (after Init/InitVoid and
// before Submit). After Submit, AttrMod returns an error.
//
// Attribute modifiers run right before rendering and can inspect or modify the
// full attribute set for the element.
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
