package utils

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// --- pool ----------------------------------------------------------------

func TestNewPoolGetPut(t *testing.T) {
	p := NewPool(func() *int {
		v := 7
		return &v
	})
	v := p.Get()
	if *v != 7 {
		t.Fatalf("Get = %d, want 7", *v)
	}
	*v = 99
	p.Put(v)
	// We can't guarantee the same pointer comes back, but we can re-Get without
	// crashing.
	_ = p.Get()
}

func TestNewStructPool(t *testing.T) {
	type box struct {
		n int
	}
	p := NewStructPool[box]()
	b := p.Get()
	if b == nil {
		t.Fatal("Get returned nil")
	}
	b.n = 5
	p.Put(b)
	_ = p.Get()
}

// --- escapedWriter -------------------------------------------------------

func TestEscapedWriterCovers(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"a&b", "a&amp;b"},
		{"\"q\"", "&#34;q&#34;"},
		{"'a'", "&#39;a&#39;"},
		{"<x>", "&lt;x&gt;"},
		{"\x00", "\uFFFD"},
		{"a<b>c&d\"e'f", "a&lt;b&gt;c&amp;d&#34;e&#39;f"},
	}
	for _, c := range cases {
		buf := &bytes.Buffer{}
		w := NewEscapedWriter(buf)
		n, err := w.Write([]byte(c.in))
		if err != nil {
			t.Fatalf("Write(%q) error: %v", c.in, err)
		}
		if n != len(c.in) {
			t.Errorf("Write(%q) n = %d, want %d", c.in, n, len(c.in))
		}
		if buf.String() != c.want {
			t.Errorf("Write(%q) = %q, want %q", c.in, buf.String(), c.want)
		}
	}
}

type errWriter struct {
	failAt int
	count  int
}

func (e *errWriter) Write(b []byte) (int, error) {
	e.count++
	if e.count == e.failAt {
		return 0, errors.New("boom")
	}
	return len(b), nil
}

func TestEscapedWriterPropagatesError(t *testing.T) {
	w := NewEscapedWriter(&errWriter{failAt: 1})
	_, err := w.Write([]byte("a&b"))
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- WriteEscapedText / WriteRawText -------------------------------------

func TestWriteEscapedText(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteEscapedText(buf, "<a>"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "&lt;a&gt;" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteRawText(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteRawText(buf, "<b>"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "<b>" {
		t.Fatalf("got %q", got)
	}
}

// --- WriteTagOpenBeg / WriteTagOpenEnd / WriteTagClose -------------------

func TestWriteTagOpenBegSanitizesName(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteTagOpenBeg(buf, "div"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "<div" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteTagOpenBegStripsBadChars(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteTagOpenBeg(buf, "di v$x!"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "<divx" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteTagOpenEnd(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteTagOpenEnd(buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != ">" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteTagClose(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteTagClose(buf, "section"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "</section>" {
		t.Fatalf("got %q", got)
	}
}

// --- WriteAttr ------------------------------------------------------------

func TestWriteAttrNilSkipped(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttr(buf, "id", nil); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("got %q, want empty", buf.String())
	}
}

func TestWriteAttrBoolFalseSkipped(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttr(buf, "disabled", false); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("got %q, want empty", buf.String())
	}
}

func TestWriteAttrBoolTrueRendersJustName(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttr(buf, "disabled", true); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "disabled" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteAttrStringValue(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttr(buf, "class", "btn primary"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `class="btn primary"` {
		t.Fatalf("got %q", got)
	}
}

func TestWriteAttrEscapesValue(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttr(buf, "data-x", "<a&b>"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `data-x="&lt;a&amp;b&gt;"` {
		t.Fatalf("got %q", got)
	}
}

func TestWriteAttrFprint(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttr(buf, "x", 42); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `x="42"` {
		t.Fatalf("got %q", got)
	}
}

// outputValue implements the Output interface so the fast-path branch in
// writeAttrValue (the one that bypasses fmt.Fprint) gets exercised.
type outputValue struct{ s string }

func (o outputValue) Output(w io.Writer) error {
	_, err := w.Write([]byte(o.s))
	return err
}

func TestWriteAttrUsesOutputInterface(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttr(buf, "x", outputValue{s: "y<z>"}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `x="y&lt;z&gt;"` {
		t.Fatalf("got %q", got)
	}
}

// errAt fails on the Nth Write call to drive the various error paths in
// writers.go.
type errAt struct {
	n     int
	count int
}

func (e *errAt) Write(b []byte) (int, error) {
	e.count++
	if e.count == e.n {
		return 0, errors.New("boom")
	}
	return len(b), nil
}

func TestWriteTagOpenBegError(t *testing.T) {
	if err := WriteTagOpenBeg(&errAt{n: 1}, "div"); err == nil {
		t.Fatal("expected error from < write")
	}
}

func TestWriteTagCloseErrors(t *testing.T) {
	if err := WriteTagClose(&errAt{n: 1}, "div"); err == nil {
		t.Fatal("expected error from </ write")
	}
	if err := WriteTagClose(&errAt{n: 3}, "div"); err == nil {
		t.Fatal("expected error from > write")
	}
}

func TestWriteAttrErrorPaths(t *testing.T) {
	// fail on writing the name
	if err := WriteAttr(&errAt{n: 1}, "id", "x"); err == nil {
		t.Fatal("expected name write error")
	}
	// fail on '='
	if err := WriteAttr(&errAt{n: 2}, "id", "x"); err == nil {
		t.Fatal("expected '=' write error")
	}
	// fail on opening quote
	if err := WriteAttr(&errAt{n: 3}, "id", "x"); err == nil {
		t.Fatal("expected opening quote error")
	}
}

// --- WriteAttrName / WriteAttrValue --------------------------------------

func TestWriteAttrName(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttrName(buf, "data-foo"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "data-foo" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteAttrValueRejectsNil(t *testing.T) {
	buf := &bytes.Buffer{}
	err := WriteAttrValue(buf, nil)
	if err == nil {
		t.Fatal("expected error for nil value")
	}
}

func TestWriteAttrValueRejectsBool(t *testing.T) {
	buf := &bytes.Buffer{}
	err := WriteAttrValue(buf, true)
	if err == nil {
		t.Fatal("expected error for bool value")
	}
}

func TestWriteAttrValueOK(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteAttrValue(buf, "v<x>"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "v&lt;x&gt;" {
		t.Fatalf("got %q, want %q", got, "v&lt;x&gt;")
	}
}

// --- Output interface override -------------------------------------------

type customOut struct{ s string }

func (c customOut) Output(w interface {
	Write([]byte) (int, error)
}) error {
	// noop adapter, real Output interface uses io.Writer.
	_, err := w.Write([]byte(c.s))
	return err
}

// We can't easily test the Output interface override here because the
// `Output` interface is io.Writer-based and not exported in a way that lets
// us shadow fmt.Fprint dispatch in tests. The fmt.Fprint path is exercised
// by TestWriteAttrFprint above.
