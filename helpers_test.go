package gox

import (
	"bytes"
	"context"
	"testing"
)

func TestEditorFunc(t *testing.T) {
	called := false
	ed := EditorFunc(func(c Cursor) error {
		called = true
		return nil
	})
	c := NewCursor(context.Background(), NewPrinter(&bytes.Buffer{}))
	if err := ed.Edit(c); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("EditorFunc.Edit not invoked")
	}
}

func TestEditorCompFunc(t *testing.T) {
	called := 0
	ec := EditorCompFunc(func(c Cursor) error {
		called++
		return c.Text("ok")
	})
	// Edit
	buf := &bytes.Buffer{}
	c := NewCursor(context.Background(), NewPrinter(buf))
	if err := ec.Edit(c); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "ok" {
		t.Fatalf("got %q", buf.String())
	}
	// Main returns an Elem that runs the same func
	buf2 := &bytes.Buffer{}
	if err := ec.Main().Render(context.Background(), buf2); err != nil {
		t.Fatal(err)
	}
	if buf2.String() != "ok" {
		t.Fatalf("got %q", buf2.String())
	}
	if called != 2 {
		t.Errorf("called = %d", called)
	}
}

func TestProxyFunc(t *testing.T) {
	called := false
	p := ProxyFunc(func(c Cursor, e Elem) error {
		called = true
		return e(c)
	})
	inner := Elem(func(c Cursor) error { return c.Text("z") })
	buf := &bytes.Buffer{}
	c := NewCursor(context.Background(), NewPrinter(buf))
	if err := p.Proxy(c, inner); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("ProxyFunc.Proxy not invoked")
	}
	if buf.String() != "z" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestModifyFunc(t *testing.T) {
	called := false
	m := ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		called = true
		return nil
	})
	if err := m.Modify(context.Background(), "div", NewAttrs()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("ModifyFunc.Modify not invoked")
	}
}

func TestPrinterFunc(t *testing.T) {
	var sent Job
	p := PrinterFunc(func(j Job) error {
		sent = j
		return nil
	})
	j := NewJobText(context.Background(), "x")
	if err := p.Send(j); err != nil {
		t.Fatal(err)
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
