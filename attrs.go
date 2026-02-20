package gox

import (
	"context"
	"io"
	"slices"
	"strings"

	"github.com/doors-dev/gox/internal/utils"
)

// Attrs is a mutable collection of element attributes.
//
// Attrs stores attributes keyed by name and keeps entries sorted lexicographically
// (by attribute name). Lookups are performed with binary search.
//
// Attribute names are case-sensitive. For example, "class" and "Class" are
// distinct entries and are stored/queried independently.
//
// Attribute presence rules (used by Attr.IsSet and Attrs.Has):
//   - nil  => not set
//   - bool => set only when true (false means “unset”)
//   - any other non-nil value => set
//
// Lifecycle notes:
//   - Attrs is intended to be built and used while constructing an element head,
//     then rendered as part of that head.
//   - Attr handles returned by Get/Find/List are references to entries inside Attrs;
//     mutating an Attr mutates the owning Attrs.
//   - Attrs is not safe for concurrent use.
type Attrs = *attrs

// AttrMod can inspect and/or mutate element attributes right before the element
// is rendered.
//
// Modifiers are executed by Attrs.ApplyMods (typically triggered during head
// rendering). They run in the order they were added and are one-shot: after
// execution, they are removed from the modifier queue.
type AttrMod interface {
	Modify(ctx context.Context, tag string, attrs Attrs) error
}

var attrsPool = utils.NewPool(func() Attrs {
	return &attrs{}
})

// NewAttrs allocates a new attribute set.
//
// The returned Attrs starts empty (no modifiers, no entries).
func NewAttrs() Attrs {
	a := attrsPool.Get()
	return a
}

type attrs struct {
	mods    []AttrMod
	entries []Attr
}

// Mutate is implemented by values that want to compute the new attribute value
// based on the previous value.
//
// Attr.Set has special handling for Mutate: if the provided value implements
// Mutate, Set calls value.Mutate(prev) where prev is the attribute’s current value,
// and stores the returned value.
type Mutate interface {
	Mutate(any) any
}

// Inherit copies all “set” attributes from attrs into a.
//
// For each attribute in attrs:
//   - if it is not set (per Attr.IsSet), it is ignored
//   - otherwise, a.Get(name).Set(value) is performed
//
// Note: because this uses Attr.Set, if the target attribute already has a value
// and the inherited value implements Mutate, the inherited value may be computed
// from the target’s previous value.
func (a Attrs) Inherit(attrs Attrs) {
	for _, attr := range attrs.entries {
		if !attr.IsSet() {
			continue
		}
		a.Get(attr.name).Set(attr.Value())
	}
}

// ApplyMods executes all queued modifiers on this attribute set.
//
// Modifiers are executed in the order they were added. Each modifier is called
// at most once; after successful completion, the modifier queue is cleared.
//
// If a modifier returns an error, ApplyMods stops immediately and returns that
// error. Modifiers that were already executed are discarded; modifiers that were
// not yet executed remain queued (until the Attrs is discarded/released).
func (a Attrs) ApplyMods(ctx context.Context, tag string) error {
	if a == nil {
		return nil
	}
	for i, m := range a.mods {
		a.mods[i] = nil
		if err := m.Modify(ctx, tag, a); err != nil {
			return err
		}
	}
	a.mods = a.mods[:0]
	return nil
}

// AddMod queues a modifier to be applied by ApplyMods.
//
// Modifiers are executed in the order they are added.
func (a Attrs) AddMod(m AttrMod) {
	a.mods = append(a.mods, m)
}

// Clone returns an independent copy of the attribute set.
//
// The returned Attrs has:
//   - a copy of the attribute entries (name/value pairs)
//   - a shallow copy of the modifier list (slice is copied; modifier values are not deep-copied)
//
// Modifying the returned Attrs does not affect the original.
func (a Attrs) Clone() Attrs {
	entries := make([]Attr, len(a.entries))
	for i := range a.entries {
		entries[i] = a.entries[i].clone()
	}
	mods := slices.Clone(a.mods)
	return &attrs{
		entries: entries,
		mods:    mods,
	}
}

// List returns a snapshot slice of all attribute entries currently tracked.
//
// The returned slice is a copy of the internal slice, but the Attr values inside
// are the same entry handles as the original Attrs. Mutating an Attr from the
// returned slice mutates the original Attrs.
func (a Attrs) List() []Attr {
	return slices.Clone(a.entries)
}

var attrPool = utils.NewStructPool[attr]()

// Has reports whether an attribute exists and is set (per Attr.IsSet).
func (a Attrs) Has(name string) bool {
	index, ok := a.search(name)
	if !ok {
		return false
	}
	attr := a.entries[index]
	return attr.IsSet()
}

// Get returns the attribute entry for name, creating it if it does not exist.
//
// Entries are kept sorted lexicographically by name; Get inserts a new entry in
// the correct position.
//
// The returned Attr is a handle into this Attrs; calling Set/Unset mutates this Attrs.
func (a Attrs) Get(name string) Attr {
	index, ok := a.search(name)
	if !ok {
		attr := attrPool.Get()
		attr.name = name
		a.entries = slices.Insert(a.entries, index, attr)
		return attr
	}
	return a.entries[index]
}

func (a *attrs) search(name string) (int, bool) {
	return slices.BinarySearchFunc(a.entries, name, func(a Attr, name string) int {
		return strings.Compare(a.name, name)
	})
}

// Find returns the attribute entry for name and whether it exists.
//
// Find does not create a new entry when the name is missing. If the attribute exists,
// it may be set or unset; use Attr.IsSet to distinguish.
func (a Attrs) Find(name string) (Attr, bool) {
	index, ok := a.search(name)
	if !ok {
		return nil, false
	}
	attr := a.entries[index]
	return attr, true
}

var space = []byte{' '}

func (a *attrs) release() {
	for i := range a.entries {
		a.entries[i] = nil
	}
	a.entries = a.entries[:0]
	for i := range a.mods {
		a.mods[i] = nil
	}
	a.mods = a.mods[:0]
	attrsPool.Put(a)
}

func (a Attrs) output(ctx context.Context, tag string, w io.Writer) error {
	if a == nil {
		return nil
	}
	if err := a.ApplyMods(ctx, tag); err != nil {
		return err
	}
	for _, attr := range a.entries {
		_, err := w.Write(space)
		if err != nil {
			return err
		}
		err = attr.output(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// Attr is a handle to a single attribute entry (name + value).
//
// Attr values are typically obtained from Attrs.Get/Find/List.
type Attr = *attr

type attr struct {
	name  string
	value any
}

// Name returns the attribute name.
func (a Attr) Name() string {
	return a.name
}

// Set sets the attribute value.
//
// Special case: if value implements Mutate, Set computes the stored value as
// value.Mutate(prev), where prev is the current stored value.
//
// Setting the value to nil unsets the attribute. Setting a bool false also results
// in the attribute being considered “unset” (see IsSet), though the stored value is false.
func (a Attr) Set(value any) {
	if v, ok := value.(Mutate); ok {
		value = v.Mutate(a.value)
	}
	a.value = value
}

// Unset clears the attribute value (equivalent to Set(nil)).
func (a Attr) Unset() {
	a.value = nil
}

// Value returns the stored value (may be nil).
func (a Attr) Value() any {
	return a.value
}

// IsSet reports whether this attribute should be considered present.
//
// Rules:
//   - nil Attr => false
//   - stored value is bool => that bool value (true=set, false=unset)
//   - otherwise => value != nil
func (a Attr) IsSet() bool {
	if a == nil {
		return false
	}
	if b, ok := a.value.(bool); ok {
		return b
	}
	return a.value != nil
}

func (a *attr) clone() Attr {
	return &attr{
		name:  a.name,
		value: a.value,
	}
}

func (a *attr) release() {
	a.Unset()
	a.name = ""
	attrPool.Put(a)
}

func (a *attr) output(w io.Writer) error {
	defer a.release()
	return utils.WriteAttr(w, a.name, a.value)
}

// OutputName writes only the attribute name to w.
//
// This is a low-level helper for custom rendering pipelines. Formatting/escaping
// are defined by GoX’s internal attribute writer.
func (a Attr) OutputName(w io.Writer) error {
	return utils.WriteAttrName(w, a.name)
}

// OutputValue writes only the attribute value to w.
//
// This is a low-level helper for custom rendering pipelines. Formatting/escaping
// are defined by GoX’s internal attribute writer.
func (a Attr) OutputValue(w io.Writer) error {
	return utils.WriteAttrValue(w, a.value)
}
