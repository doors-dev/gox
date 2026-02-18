package gox

import (
	"context"
	"io"
	"slices"
	"strings"

	"github.com/doors-dev/gox/internal/utils"
)

type Attrs = *attrs

type AttrMod interface {
	Modify(ctx context.Context, tag string, attrs Attrs) error
}

var attrsPool = utils.NewPool(func() Attrs {
	return &attrs{}
})

func NewAttrs() Attrs {
	a := attrsPool.Get()
	return a
}

type attrs struct {
	mods    []AttrMod
	entries []Attr
}

type Mutate interface {
	Mutate(any) any
}

func (a Attrs) Inherit(attrs Attrs) {
	for _, attr := range attrs.entries {
		if !attr.IsSet() {
			continue
		}
		a.Get(attr.name).Set(attr.Value())
	}
}

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

func (a Attrs) AddMod(m AttrMod) {
	a.mods = append(a.mods, m)
}

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

func (a Attrs) List() []Attr {
	return slices.Clone(a.entries)
}

var attrPool = utils.NewStructPool[attr]()

func (a Attrs) Has(name string) bool {
	index, ok := a.search(name)
	if !ok {
		return false
	}
	attr := a.entries[index]
	return attr.IsSet()
}

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

type Attr = *attr

type attr struct {
	name  string
	value any
}

func (a Attr) Name() string {
	return a.name
}

func (a Attr) Set(value any) {
	if v, ok := value.(Mutate); ok {
		value = v.Mutate(a.value)
	}
	a.value = value
}

func (a Attr) Unset() {
	a.value = nil
}

func (a Attr) Value() any {
	return a.value
}

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

func (a Attr) OutputName(w io.Writer) error {
	return utils.WriteAttrName(w, a.name)
}

func (a Attr) OutputValue(w io.Writer) error {
	return utils.WriteAttrValue(w, a.value)
}
