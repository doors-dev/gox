package gox

import (
	"context"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/doors-dev/gox/utils"
)

type Attrs = *attrs

type AttrMod interface {
	Apply(ctx context.Context, attrs Attrs) error
}

var attrsPool = utils.NewPool(func() Attrs {
	return &attrs{}
})

func NewAttrs(ctx context.Context) Attrs {
	a := attrsPool.Get()
	a.Ctx = ctx
	return a
}

type attrs struct {
	Ctx     context.Context
	mods    []AttrMod
	entries []Attr
}

func (a Attrs) ApplyMods() error {
	if a == nil {
		return nil
	}
	for i, m := range a.mods {
		a.mods[i] = nil
		if err := m.Apply(a.Ctx, a); err != nil {
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
		Ctx:     a.Ctx,
		entries: entries,
		mods:    mods,
	}
}

func (a Attrs) List() []Attr {
	return slices.Clone(a.entries)
}

var attrPool = utils.NewStructPool[attr]()

func (a Attrs) Get(name string) Attr {
	index, ok := slices.BinarySearchFunc(a.entries, name, func(a Attr, name string) int {
		return strings.Compare(a.name, name)
	})
	if !ok {
		attr := attrPool.Get()
		attr.name = name
		a.entries = slices.Insert(a.entries, index, attr)
		return attr
	}
	return a.entries[index]
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
	a.Ctx = nil
	attrsPool.Put(a)
}

func (a Attrs) output(w io.Writer) error {
	if a == nil {
		return nil
	}
	if err := a.ApplyMods(); err != nil {
		return err
	}
	for _, attr := range a.entries {
		_, err := w.Write(space)
		if err != nil {
			return err
		}
		err = attr.Render(w)
		if err != nil {
			return err
		}
	}
	return nil
}

type Attr = *attr

type attr struct {
	name  string
	value attrValue
}

type attrValue interface {
	render(w io.Writer, name string) error
	clone() attrValue
}

func (a *attr) clone() Attr {
	if a.value == nil {
		return &attr{
			name:  a.name,
			value: nil,
		}
	}
	return &attr{
		name:  a.name,
		value: a.value.clone(),
	}
}

func (a Attr) IsSet() bool {
	return a.value != nil
}

func (a *attr) release() {
	a.name = ""
	a.value = nil
	attrPool.Put(a)
}

func (a Attr) Render(w io.Writer) error {
	defer a.release()
	if a.value == nil {
		return nil
	}
	return a.value.render(w, a.name)
}

func (a Attr) Name() string {
	return a.name
}

func (a Attr) Set(value string) {
	a.releasePrevValue()
	a.value = newStringValue(value)
}

func (a Attr) Append(value string) {
	v, ok := a.value.(*stringValue)
	if !ok {
		a.Set(value)
		return
	}
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return
	}
	v.append(value)
}

func (a Attr) SetBool(value bool) {
	a.releasePrevValue()
	if value {
		a.value = boolValue{}
		return
	}
	a.value = nil
}

func (a Attr) SetObject(value any) {
	a.releasePrevValue()
	a.value = newJsonValue(value)
}

func (a Attr) AppendObject(value any) {
	v, ok := a.value.(*jsonArrayValue)
	if !ok {
		a.releasePrevValue()
		a.value = newJsonArrayValue(value)
		return
	}
	v.append(value)
}

func (a Attr) releasePrevValue() {
	if a.value == nil {
		return
	}
	_, ok := a.value.(boolValue)
	if ok {
		return
	}
	s, ok := a.value.(*stringValue)
	if ok {
		s.release()
		return
	}
	v, ok := a.value.(*jsonValue)
	if ok {
		v.release()
		return
	}
	va, ok := a.value.(*jsonArrayValue)
	if ok {
		va.release()
		return
	}
}

func (a Attr) ReadBool() (bool, bool) {
	if a.value == nil {
		return false, true
	}
	_, ok := a.value.(boolValue)
	return true, ok
}

func (a Attr) ReadObject() (any, bool) {
	v, ok := a.value.(*jsonValue)
	return v.value, ok
}

func (a Attr) ReadObjectArray() ([]any, bool) {
	v, ok := a.value.(*jsonArrayValue)
	return *v, ok
}

func (a Attr) ReadString() (string, bool) {
	v, ok := a.value.(*stringValue)
	return v.string(), ok
}

type boolValue struct{}

func (v boolValue) clone() attrValue {
	return &boolValue{}
}

func (v boolValue) render(w io.Writer, name string) error {
	return utils.WriteBoolAttr(w, name)
}

var jsonValuePool = utils.NewPool(func() *jsonValue {
	v := jsonValue{}
	return &v
})

func newJsonValue(value any) *jsonValue {
	v := jsonValuePool.Get()
	v.value = value
	return v
}

type jsonValue struct {
	value any
}

func (v *jsonValue) release() {
	v.value = nil
	jsonValuePool.Put(v)
}

func (v *jsonValue) clone() attrValue {
	return newJsonValue(v.value)
}

func (v *jsonValue) render(w io.Writer, name string) error {
	defer v.release()
	return utils.WriteAttrJson(w, name, v.value)
}

var jsonArrayAttrPool = utils.NewPool(func() *jsonArrayValue {
	v := make(jsonArrayValue, 1)
	return &v
})

func newJsonArrayValue(value any) *jsonArrayValue {
	v := jsonArrayAttrPool.Get()
	(*v)[0] = value
	return v
}

type jsonArrayValue []any

func (v *jsonArrayValue) release() {
	for i := range *v {
		(*v)[i] = nil
	}
	*v = (*v)[:1]
	jsonArrayAttrPool.Put(v)
}

func (v *jsonArrayValue) clone() attrValue {
	clone := jsonArrayValue(slices.Clone(*v))
	return &clone
}

func (v *jsonArrayValue) render(w io.Writer, name string) error {
	defer v.release()
	return utils.WriteAttrJson(w, name, *v)
}

func (v *jsonArrayValue) append(value any) {
	*v = append(*v, value)
}

var stringAttrPool = utils.NewPool(func() *stringValue {
	v := make(stringValue, 1)
	return &v
})

func newStringValue(value string) *stringValue {
	v := stringAttrPool.Get()
	(*v)[0] = value
	return v
}

type stringValue []string

func (v *stringValue) release() {
	for i, _ := range *v {
		(*v)[i] = ""
	}
	*v = (*v)[:1]
	stringAttrPool.Put(v)
}

func (v *stringValue) clone() attrValue {
	clone := stringValue(slices.Clone(*v))
	return &clone
}

func (v *stringValue) render(w io.Writer, name string) error {
	defer v.release()
	return utils.WriteAttr(w, name, *v)
}

func (v *stringValue) append(value string) {
	*v = append(*v, value)
}

func (v *stringValue) string() string {
	if len(*v) == 1 {
		return (*v)[0]
	}
	build := strings.Builder{}
	prevSpace := true
	for _, v := range *v {
		if !prevSpace {
			spaced := unicode.IsSpace(rune(v[0]))
			if !spaced {
				build.WriteByte(' ')
			}
		}
		build.WriteString(v)
	}
	return build.String()
}
