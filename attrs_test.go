package gox

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"testing"
)

type errWriter struct {
	failAt int
	writes int
	err    error
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAt {
		return 0, w.err
	}
	w.writes++
	return len(p), nil
}

type errOutput struct{ err error }

func (o errOutput) Output(io.Writer) error {
	return o.err
}

func TestNewAttrsEmpty(t *testing.T) {
	a := NewAttrs()
	if a == nil {
		t.Fatal("NewAttrs returned nil")
	}
	if got := a.List(); len(got) != 0 {
		t.Fatalf("NewAttrs().List() len = %d, want 0 (%#v)", len(got), got)
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
	if got[0].Name() != "class" || got[1].Name() != "id" {
		t.Fatalf("List = %v %v", got[0].Name(), got[1].Name())
	}
}

func TestAttrsClone(t *testing.T) {
	a := NewAttrs()
	a.Get("id").Set("x")
	a.Get("hidden")
	c := a.Clone()
	if !c.Has("id") {
		t.Fatal("clone missing id")
	}
	if c.Has("hidden") {
		t.Fatal("clone copied unset attr")
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
	at.Set(`a&"b`)
	buf := &bytes.Buffer{}
	if err := at.OutputValue(buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "a&amp;&#34;b" {
		t.Fatalf("OutputValue = %q, want HTML-escaped value", got)
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

func TestAttrsApplyModsErrorKeepsOnlyPending(t *testing.T) {
	a := NewAttrs()
	var order []int
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		order = append(order, 1)
		return nil
	}))
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		order = append(order, 2)
		return errors.New("boom")
	}))
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		order = append(order, 3)
		return nil
	}))
	if err := a.ApplyMods(context.Background(), "div"); err == nil {
		t.Fatal("expected error")
	}
	if !slices.Equal(order, []int{1, 2}) {
		t.Fatalf("order after error = %v", order)
	}
	if err := a.ApplyMods(context.Background(), "div"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []int{1, 2, 3}) {
		t.Fatalf("order after retry = %v", order)
	}
}

func TestAttrsApplyModsRunsModAddedDuringApply(t *testing.T) {
	a := NewAttrs()
	var order []int
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		order = append(order, 1)
		attrs.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
			order = append(order, 3)
			return nil
		}))
		return nil
	}))
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		order = append(order, 2)
		return nil
	}))
	if err := a.ApplyMods(context.Background(), "div"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []int{1, 2, 3}) {
		t.Fatalf("order = %v", order)
	}
	if err := a.ApplyMods(context.Background(), "div"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []int{1, 2, 3}) {
		t.Fatalf("order after second apply = %v", order)
	}
}

func TestAttrsApplyModsRunsNestedAdditions(t *testing.T) {
	a := NewAttrs()
	var order []int
	a.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		order = append(order, 1)
		attrs.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
			order = append(order, 2)
			attrs.AddMod(ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
				order = append(order, 3)
				return nil
			}))
			return nil
		}))
		return nil
	}))
	if err := a.ApplyMods(context.Background(), "div"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []int{1, 2, 3}) {
		t.Fatalf("order = %v", order)
	}
}

func TestAttrsOutputNilIsNoop(t *testing.T) {
	var a Attrs
	buf := &bytes.Buffer{}
	if err := a.output(context.Background(), "div", buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "" {
		t.Fatalf("nil output wrote %q, want empty output", got)
	}
}

func TestAttrsOutputPropagatesSpaceWriteError(t *testing.T) {
	a := NewAttrs()
	a.Get("class").Set("c")
	w := &errWriter{failAt: 0, err: errors.New("space write failed")}
	err := a.output(context.Background(), "div", w)
	if err == nil || err.Error() != "space write failed" {
		t.Fatalf("output error = %v", err)
	}
}

func TestAttrsOutputPropagatesAttrError(t *testing.T) {
	a := NewAttrs()
	want := errors.New("attr output failed")
	a.Get("data-x").Set(errOutput{err: want})
	err := a.output(context.Background(), "div", &bytes.Buffer{})
	if !errors.Is(err, want) {
		t.Fatalf("output error = %v, want %v", err, want)
	}
}
