package gox

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestNewAttrsEmpty(t *testing.T) {
	a := NewAttrs()
	if a == nil {
		t.Fatal("NewAttrs returned nil")
	}
	if a.Has("id") {
		t.Fatal("empty Attrs.Has = true")
	}
}

func TestAttrsGetSetValue(t *testing.T) {
	a := NewAttrs()
	a.Get("id").Set("x")
	if !a.Has("id") {
		t.Fatal("Has after Set = false")
	}
	at, ok := a.Find("id")
	if !ok {
		t.Fatal("Find = !ok")
	}
	if at.Name() != "id" {
		t.Errorf("Name = %q", at.Name())
	}
	if at.Value() != "x" {
		t.Errorf("Value = %v", at.Value())
	}
	if !at.IsSet() {
		t.Error("IsSet = false")
	}
}

func TestAttrsBoolFalseUnset(t *testing.T) {
	a := NewAttrs()
	a.Get("disabled").Set(false)
	if a.Has("disabled") {
		t.Fatal("Has(false bool) = true, want false")
	}
}

func TestAttrsBoolTrueSet(t *testing.T) {
	a := NewAttrs()
	a.Get("disabled").Set(true)
	if !a.Has("disabled") {
		t.Fatal("Has(true bool) = false")
	}
}

func TestAttrsFindMissing(t *testing.T) {
	a := NewAttrs()
	if _, ok := a.Find("nope"); ok {
		t.Fatal("Find missing = ok")
	}
}

func TestAttrsFindUnsetReturnsFalse(t *testing.T) {
	a := NewAttrs()
	a.Get("id") // creates entry but doesn't set
	if _, ok := a.Find("id"); ok {
		t.Fatal("Find unset = ok")
	}
}

func TestAttrsList(t *testing.T) {
	a := NewAttrs()
	a.Get("id").Set("x")
	a.Get("class").Set("c")
	got := a.List()
	if len(got) != 2 {
		t.Fatalf("List len = %d", len(got))
	}
	// Sorted by name: class < id
	if got[0].Name() != "class" || got[1].Name() != "id" {
		t.Fatalf("List = %v %v", got[0].Name(), got[1].Name())
	}
}

func TestAttrsClone(t *testing.T) {
	a := NewAttrs()
	a.Get("id").Set("x")
	c := a.Clone()
	if !c.Has("id") {
		t.Fatal("clone missing id")
	}
	c.Get("id").Set("y")
	if v, _ := a.Find("id"); v.Value() != "x" {
		t.Fatalf("clone shared state: %v", v.Value())
	}
}

func TestAttrsInherit(t *testing.T) {
	src := NewAttrs()
	src.Get("id").Set("x")
	src.Get("class").Set("c")
	src.Get("hidden").Set(false) // unset, must be skipped

	dst := NewAttrs()
	dst.Inherit(src)
	if !dst.Has("id") || !dst.Has("class") {
		t.Fatal("Inherit missing attrs")
	}
	if dst.Has("hidden") {
		t.Fatal("Inherit copied unset attr")
	}
}

type mutateValue struct{ v string }

func (m mutateValue) Mutate(name string, prev any) any {
	if prev == nil {
		return m.v
	}
	return prev.(string) + "+" + m.v
}

func TestAttrSetMutate(t *testing.T) {
	a := NewAttrs()
	a.Get("class").Set("first")
	a.Get("class").Set(mutateValue{v: "second"})
	at, _ := a.Find("class")
	if at.Value() != "first+second" {
		t.Fatalf("Mutate = %v", at.Value())
	}
}

func TestAttrUnset(t *testing.T) {
	a := NewAttrs()
	at := a.Get("id")
	at.Set("x")
	at.Unset()
	if at.IsSet() {
		t.Fatal("Unset.IsSet = true")
	}
}

func TestAttrIsSetNil(t *testing.T) {
	var a Attr
	if a.IsSet() {
		t.Fatal("nil Attr.IsSet = true")
	}
}

func TestAttrOutputName(t *testing.T) {
	a := NewAttrs()
	at := a.Get("data-x")
	at.Set("v")
	buf := &bytes.Buffer{}
	if err := at.OutputName(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "data-x" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestAttrOutputValue(t *testing.T) {
	a := NewAttrs()
	at := a.Get("class")
	at.Set("c")
	buf := &bytes.Buffer{}
	if err := at.OutputValue(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() == "" {
		t.Fatal("empty output")
	}
}

func TestAttrsApplyModsNil(t *testing.T) {
	var a Attrs
	if err := a.ApplyMods(context.Background(), "div"); err != nil {
		t.Fatal(err)
	}
}

func TestAttrsApplyModsRunsAndClears(t *testing.T) {
	a := NewAttrs()
	called := 0
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		called++
		return nil
	}))
	if err := a.ApplyMods(context.Background(), "div"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Errorf("called = %d", called)
	}
	// Re-applying should be a no-op (modifiers cleared).
	if err := a.ApplyMods(context.Background(), "div"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Errorf("after second apply called = %d", called)
	}
}

func TestAttrsApplyModsErrorStops(t *testing.T) {
	a := NewAttrs()
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		return errors.New("boom")
	}))
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		t.Error("second modifier should not run")
		return nil
	}))
	if err := a.ApplyMods(context.Background(), "div"); err == nil {
		t.Fatal("expected error")
	}
}
