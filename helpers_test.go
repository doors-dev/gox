package gox

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestProxyFunc(t *testing.T) {
	buf := &bytes.Buffer{}
	cur := NewCursor(context.Background(), NewPrinter(buf))
	innerRan := false
	inner := Elem(func(c Cursor) error {
		innerRan = true
		return c.Text("z")
	})
	want := errors.New("boom")
	p := ProxyFunc(func(got Cursor, elem Elem) error {
		if got != cur {
			t.Fatalf("Proxy() cursor = %v, want %v", got, cur)
		}
		if err := elem(got); err != nil {
			t.Fatalf("inner elem error = %v", err)
		}
		return want
	})
	if err := p.Proxy(cur, inner); !errors.Is(err, want) {
		t.Fatalf("Proxy() error = %v, want %v", err, want)
	}
	if buf.String() != "z" {
		t.Fatalf("got %q", buf.String())
	}
	if !innerRan {
		t.Fatal("Proxy() elem did not run")
	}
}

func TestModifyFunc(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "value")
	attrs := NewAttrs()
	want := errors.New("boom")
	m := ModifyFunc(func(gotCtx context.Context, tag string, gotAttrs Attrs) error {
		if gotCtx != ctx {
			t.Fatal("Modify() context did not round-trip")
		}
		if tag != "div" {
			t.Fatalf("Modify() tag = %q, want %q", tag, "div")
		}
		if gotAttrs != attrs {
			t.Fatal("Modify() attrs did not round-trip")
		}
		gotAttrs.Get("id").Set("from-modifier")
		return want
	})
	if err := m.Modify(ctx, "div", attrs); !errors.Is(err, want) {
		t.Fatalf("Modify() error = %v, want %v", err, want)
	}
	if got, ok := attrs.Find("id"); !ok || got.Value() != "from-modifier" {
		t.Fatalf("attrs after Modify() = (%v, %v), want id=from-modifier", got, ok)
	}
}

func TestPrinterFunc(t *testing.T) {
	want := errors.New("boom")
	var sent Job
	p := PrinterFunc(func(j Job) error {
		sent = j
		return want
	})
	j := NewJobText(context.Background(), "x")
	if err := p.Send(j); !errors.Is(err, want) {
		t.Fatalf("Send() error = %v, want %v", err, want)
	}
	if sent != j {
		t.Fatal("PrinterFunc did not pass job through")
	}
	// release the unsent job to keep the pool clean
	_ = j.Output(&bytes.Buffer{})
}

func TestNewEscapedWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewEscapedWriter(buf)
	if _, err := w.Write([]byte("<a>")); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "&lt;a&gt;" {
		t.Fatalf("got %q", buf.String())
	}
}
