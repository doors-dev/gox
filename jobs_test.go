package gox

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestJobHeadOpenContainerNoOutput(t *testing.T) {
	a := NewAttrs()
	a.Get("class").Set("ignored")
	j := NewJobHeadOpen(context.Background(), 1, KindContainer, "div", a)
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("container output: %q", buf.String())
	}
}

func TestJobHeadOpenEmptyTagErrors(t *testing.T) {
	j := NewJobHeadOpen(context.Background(), 1, KindRegular, "", nil)
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err == nil {
		t.Fatal("expected error")
	}
}

func TestJobHeadOpenWithAttrs(t *testing.T) {
	a := NewAttrs()
	a.Get("class").Set("c")
	a.Get("id").Set("x")
	j := NewJobHeadOpen(context.Background(), 1, KindRegular, "div", a)
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `<div class="c" id="x">` {
		t.Fatalf("got %q", got)
	}
}

func TestJobHeadOpenNilAttrs(t *testing.T) {
	j := NewJobHeadOpen(context.Background(), 1, KindRegular, "div", nil)
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "<div>" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestJobHeadOpenWriteError(t *testing.T) {
	want := errors.New("write failed")
	j := NewJobHeadOpen(context.Background(), 1, KindRegular, "div", nil)
	if err := j.Output(&errWriter{failAt: 0, err: want}); !errors.Is(err, want) {
		t.Fatalf("Output() error = %v, want %v", err, want)
	}
}

func TestJobHeadCloseContainerNoOutput(t *testing.T) {
	j := NewJobHeadClose(context.Background(), 1, KindContainer, "")
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatal("container close had output")
	}
}

func TestJobHeadCloseVoidErrors(t *testing.T) {
	j := NewJobHeadClose(context.Background(), 1, KindVoid, "input")
	if err := j.Output(&bytes.Buffer{}); err == nil {
		t.Fatal("expected error closing void")
	}
}

func TestJobHeadCloseEmptyTagErrors(t *testing.T) {
	j := NewJobHeadClose(context.Background(), 1, KindRegular, "")
	if err := j.Output(&bytes.Buffer{}); err == nil {
		t.Fatal("expected error empty tag")
	}
}

func TestJobHeadCloseRegular(t *testing.T) {
	j := NewJobHeadClose(context.Background(), 1, KindRegular, "section")
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "</section>" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestJobHeadOpenContext(t *testing.T) {
	type k struct{}
	ctx := context.WithValue(context.Background(), k{}, "v")
	j := NewJobHeadOpen(ctx, 1, KindContainer, "", nil)
	if j.Context().Value(k{}) != "v" {
		t.Fatal("context not propagated")
	}
	_ = j.Output(&bytes.Buffer{})
}

func TestJobHeadCloseContext(t *testing.T) {
	ctx := context.Background()
	j := NewJobHeadClose(ctx, 1, KindContainer, "")
	if j.Context() != ctx {
		t.Fatal("ctx mismatch")
	}
	_ = j.Output(&bytes.Buffer{})
}

func TestJobTextOutput(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("job"), "text")
	j := NewJobText(ctx, "<x>")
	if j.Context() != ctx {
		t.Fatal("Context() did not round-trip the provided context")
	}
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "&lt;x&gt;" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestJobRawOutput(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("job"), "raw")
	j := NewJobRaw(ctx, "<x>")
	if j.Context() != ctx {
		t.Fatal("Context() did not round-trip the provided context")
	}
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "<x>" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestJobBytesOutput(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("job"), "bytes")
	j := NewJobBytes(ctx, []byte("hi"))
	if j.Context() != ctx {
		t.Fatal("Context() did not round-trip the provided context")
	}
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hi" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestJobFprintOutput(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("job"), "fprint")
	j := NewJobFprint(ctx, 42)
	if j.Context() != ctx {
		t.Fatal("Context() did not round-trip the provided context")
	}
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "42" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestJobTemplOutput(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("job"), "templ")
	j := NewJobTempl(ctx, templStub{s: "tt"})
	if j.Context() != ctx {
		t.Fatal("Context() did not round-trip the provided context")
	}
	buf := &bytes.Buffer{}
	if err := j.Output(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "tt" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestJobErrorOutput(t *testing.T) {
	want := errors.New("kaboom")
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("job"), "error")
	j := NewJobError(ctx, want)
	if j.Context() != ctx {
		t.Fatal("Context() did not round-trip the provided context")
	}
	if err := j.Output(&bytes.Buffer{}); err != want {
		t.Fatalf("got %v", err)
	}
}

func TestOutputErrorString(t *testing.T) {
	if got := OutputError("boom").Error(); got != "boom" {
		t.Fatalf("OutputError.Error() = %q, want %q", got, "boom")
	}
}

type fakeReleaser struct{ released bool }

func (f *fakeReleaser) release() { f.released = true }

func TestRelease(t *testing.T) {
	r := &fakeReleaser{}
	Release(r)
	if !r.released {
		t.Fatal("Release did not call release()")
	}
}
