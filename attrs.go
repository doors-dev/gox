package gox

import (
	"context"
	"io"
	"slices"
	"strings"

	"github.com/doors-dev/gox/internal/utils"
)

// Attrs stores the attributes attached to one element head.
//
// Example:
//
//	attrs := gox.NewAttrs()
//	attrs.Get("class").Set("badge")
//	attrs.Get("hidden").Set(false)
//
// Attrs keeps entries sorted by name for stable output and binary-search
// lookups. Names are case-sensitive, so "class" and "Class" are different
// attributes.
//
// Presence follows these rules:
//   - nil means unset
//   - bool is set only when true
//   - any other non-nil value is set
//
// Attr handles returned by Get, Find, and List point into the owning Attrs.
// Attrs is not safe for concurrent use.
type Attrs = *attrs

// Modify can inspect and mutate an element's full attribute set
// right before rendering.
//
// Modifiers run in the order they were added. They are one-shot: after
// ApplyMods succeeds, the queue is cleared.
type Modify interface {
	Modify(ctx context.Context, tag string, attrs Attrs) error
}

var attrsPool = utils.NewPool(func() Attrs {
	return &attrs{}
})

// NewAttrs returns an empty attribute set.
func NewAttrs() Attrs {
	a := attrsPool.Get()
	return a
}

type attrs struct {
	mods    []Modify
	entries []Attr
}

// Mutate computes a new attribute value from the previous one.
//
// Attr.Set detects Mutate values and stores the result of
// value.Mutate(attributeName, currentValue).
type Mutate interface {
	Mutate(name string, value any) any
}

// Inherit copies all set attributes from attrs into a.
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

// ApplyMods runs all queued modifiers on this attribute set.
//
// Modifiers run in insertion order. If one returns an error, ApplyMods stops
// immediately. Modifiers that already ran are discarded.
func (a Attrs) ApplyMods(ctx context.Context, tag string) error {
	if a == nil {
		return nil
	}
	for len(a.mods) > 0 {
		m := a.mods[0]
		a.mods[0] = nil
		a.mods = a.mods[1:]
		if err := m.Modify(ctx, tag, a); err != nil {
			return err
		}
	}
	return nil
}

// AddMod queues m to run during ApplyMods.
func (a Attrs) AddMod(m Modify) {
	a.mods = append(a.mods, m)
}

// Clone returns an independent copy of the attribute set.
//
// Clone copies only attributes that are currently set. The modifier slice is
// copied too, but modifier values are shared.
func (a Attrs) Clone() Attrs {
	entries := make([]Attr, 0, len(a.entries))
	for _, e := range a.entries {
		if !e.IsSet() {
			continue
		}
		entries = append(entries, e.clone())
	}
	mods := slices.Clone(a.mods)
	return &attrs{
		entries: entries,
		mods:    mods,
	}
}

// List returns a snapshot of all tracked attribute entries.
//
// The slice itself is copied, but each Attr still points at the original entry.
// List includes unset attributes.
func (a Attrs) List() []Attr {
	return slices.Clone(a.entries)
}

var attrPool = utils.NewStructPool[attr]()

// Has reports whether name exists and is currently set.
func (a Attrs) Has(name string) bool {
	index, ok := a.search(name)
	if !ok {
		return false
	}
	attr := a.entries[index]
	return attr.IsSet()
}

// Get returns the entry for name, creating it when needed.
//
// The returned Attr is a live handle into this Attrs.
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

// Find returns the set attribute named name.
//
// Find does not create missing entries and returns false for unset ones.
func (a Attrs) Find(name string) (Attr, bool) {
	index, ok := a.search(name)
	if !ok {
		return nil, false
	}
	attr := a.entries[index]
	if !attr.IsSet() {
		return nil, false
	}
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
		if !attr.IsSet() {
			continue
		}
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

// Attr is a handle to one attribute entry.
//
// Attr values usually come from Attrs.Get, Attrs.Find, or Attrs.List.
type Attr = *attr

type attr struct {
	name  string
	value any
}

// Name returns the attribute name.
func (a Attr) Name() string {
	return a.name
}

// Set stores value in the attribute.
//
// If value implements Mutate, Set stores the result of
// value.Mutate(attributeName, currentValue). Nil unsets the attribute. A bool
// false is stored but still treated as unset by IsSet.
func (a Attr) Set(value any) {
	if v, ok := value.(Mutate); ok {
		value = v.Mutate(a.name, a.value)
	}
	a.value = value
}

// Unset clears the attribute value.
func (a Attr) Unset() {
	a.value = nil
}

// Value returns the stored value, which may be nil.
func (a Attr) Value() any {
	return a.value
}

// IsSet reports whether this attribute should be rendered.
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
// This is a low-level helper for custom rendering pipelines.
func (a Attr) OutputName(w io.Writer) error {
	return utils.WriteAttrName(w, a.name)
}

// OutputValue writes only the attribute value to w.
//
// This is a low-level helper for custom rendering pipelines.
func (a Attr) OutputValue(w io.Writer) error {
	return utils.WriteAttrValue(w, a.value)
}
