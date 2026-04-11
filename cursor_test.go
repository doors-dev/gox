package gox

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// renderElem runs e through a default printer and returns its output.
func renderElem(t *testing.T, e Elem) string {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := e.Render(context.Background(), buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return buf.String()
}

// --- Cursor regular element ---------------------------------------------

func TestCursorRegularElement(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		if err := c.Init("span"); err != nil {
			return err
		}
		if err := c.AttrSet("class", "badge"); err != nil {
			return err
		}
		if err := c.Submit(); err != nil {
			return err
		}
		if err := c.Text("hi"); err != nil {
			return err
		}
		return c.Close()
	})
	want := `<span class="badge">hi</span>`
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestCursorVoidElement(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		if err := c.InitVoid("input"); err != nil {
			return err
		}
		if err := c.AttrSet("disabled", true); err != nil {
			return err
		}
		return c.Submit()
	})
	if !strings.Contains(out, "<input") || !strings.Contains(out, "disabled") {
		t.Fatalf("got %q", out)
	}
}

func TestCursorContainer(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		if err := c.InitContainer(); err != nil {
			return err
		}
		if err := c.Text("a"); err != nil {
			return err
		}
		if err := c.Text("b"); err != nil {
			return err
		}
		return c.Close()
	})
	if out != "ab" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorTextEscapes(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Text("<a&b>")
	})
	if !strings.Contains(out, "&lt;a&amp;b&gt;") {
		t.Fatalf("got %q", out)
	}
}

func TestCursorRawNoEscape(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Raw("<b>")
	})
	if out != "<b>" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorBytesNoEscape(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Bytes([]byte("<i>"))
	})
	if out != "<i>" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorFprint(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Fprint(42)
	})
	if out != "42" {
		t.Fatalf("got %q", out)
	}
}

// --- Cursor.Any dispatch -----------------------------------------------

func TestCursorAnyNil(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any(nil)
	})
	if out != "" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyString(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any("foo<")
	})
	if !strings.Contains(out, "foo&lt;") {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyStringSlice(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any([]string{"a", "b"})
	})
	if out != "ab" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyElem(t *testing.T) {
	inner := Elem(func(c Cursor) error { return c.Text("inner") })
	out := renderElem(t, func(c Cursor) error {
		return c.Any(inner)
	})
	if out != "inner" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyElemSlice(t *testing.T) {
	a := Elem(func(c Cursor) error { return c.Text("a") })
	b := Elem(func(c Cursor) error { return c.Text("b") })
	out := renderElem(t, func(c Cursor) error {
		return c.Any([]Elem{a, b})
	})
	if out != "ab" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyComp(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any(Elem(func(c Cursor) error { return c.Text("c") }))
	})
	if out != "c" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyCompSlice(t *testing.T) {
	a := Elem(func(c Cursor) error { return c.Text("a") })
	b := Elem(func(c Cursor) error { return c.Text("b") })
	out := renderElem(t, func(c Cursor) error {
		return c.Any([]Comp{a, b})
	})
	if out != "ab" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyEditor(t *testing.T) {
	ed := EditorFunc(func(c Cursor) error { return c.Text("e") })
	out := renderElem(t, func(c Cursor) error {
		return c.Any(ed)
	})
	if out != "e" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyAnySlice(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any([]any{"a", 1, "b"})
	})
	if out != "a1b" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyFallback(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any(3.14)
	})
	if out != "3.14" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorMany(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Many("a", 1, "b")
	})
	if out != "a1b" {
		t.Fatalf("got %q", out)
	}
}

// --- State machine errors ---------------------------------------------

func TestCursorTextWithoutSubmitErrors(t *testing.T) {
	err := Elem(func(c Cursor) error {
		if err := c.Init("span"); err != nil {
			return err
		}
		// Forgot Submit
		return c.Text("oops")
	}).Render(context.Background(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error from text-before-submit")
	}
}

func TestCursorDoubleSubmitErrors(t *testing.T) {
	err := Elem(func(c Cursor) error {
		if err := c.Init("span"); err != nil {
			return err
		}
		if err := c.Submit(); err != nil {
			return err
		}
		return c.Submit()
	}).Render(context.Background(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error from double submit")
	}
}

func TestCursorCloseWithoutInitErrors(t *testing.T) {
	err := Elem(func(c Cursor) error {
		return c.Close()
	}).Render(context.Background(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error from close-without-init")
	}
}

func TestCursorAttrAfterSubmitErrors(t *testing.T) {
	err := Elem(func(c Cursor) error {
		if err := c.Init("span"); err != nil {
			return err
		}
		if err := c.Submit(); err != nil {
			return err
		}
		return c.AttrSet("id", "x")
	}).Render(context.Background(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error setting attr after submit")
	}
}

// --- Cursor.NewID / Context --------------------------------------------

func TestCursorNewIDIncreases(t *testing.T) {
	c := NewCursor(context.Background(), NewPrinter(&bytes.Buffer{}))
	a := c.NewID()
	b := c.NewID()
	if b <= a {
		t.Fatalf("ids not increasing: %d %d", a, b)
	}
}

func TestCursorContext(t *testing.T) {
	type k struct{}
	ctx := context.WithValue(context.Background(), k{}, "v")
	c := NewCursor(ctx, NewPrinter(&bytes.Buffer{}))
	if c.Context().Value(k{}) != "v" {
		t.Fatal("Context did not propagate value")
	}
}

// --- HeadError -----------------------------------------------------------

func TestHeadErrorString(t *testing.T) {
	if HeadError("boom").Error() != "boom" {
		t.Fatal("HeadError")
	}
}

// --- AttrMod -------------------------------------------------------------

func TestCursorAttrMod(t *testing.T) {
	mod := ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		attrs.Get("data-tag").Set(tag)
		return nil
	})
	out := renderElem(t, func(c Cursor) error {
		if err := c.Init("section"); err != nil {
			return err
		}
		if err := c.AttrMod(mod); err != nil {
			return err
		}
		if err := c.Submit(); err != nil {
			return err
		}
		return c.Close()
	})
	if !strings.Contains(out, `data-tag="section"`) {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAttrModError(t *testing.T) {
	mod := ModifyFunc(func(ctx context.Context, tag string, attrs Attrs) error {
		return errors.New("boom")
	})
	err := Elem(func(c Cursor) error {
		if err := c.Init("section"); err != nil {
			return err
		}
		if err := c.AttrMod(mod); err != nil {
			return err
		}
		if err := c.Submit(); err != nil {
			return err
		}
		return c.Close()
	}).Render(context.Background(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected mod error")
	}
}

func TestCursorAttrModAfterSubmitErrors(t *testing.T) {
	err := Elem(func(c Cursor) error {
		if err := c.Init("span"); err != nil {
			return err
		}
		if err := c.Submit(); err != nil {
			return err
		}
		return c.AttrMod(ModifyFunc(func(ctx context.Context, tag string, a Attrs) error { return nil }))
	}).Render(context.Background(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Send / Comp / Templ ------------------------------------------------

func TestCursorSendBypassesValidation(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Send(NewJobText(c.Context(), "hi"))
	})
	if out != "hi" {
		t.Fatalf("got %q", out)
	}
}

type compStub struct{ s string }

func (c compStub) Main() Elem {
	return Elem(func(cur Cursor) error { return cur.Text(c.s) })
}

func TestCursorComp(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Comp(compStub{s: "ok"})
	})
	if out != "ok" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorCompCtx(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.CompCtx(context.Background(), compStub{s: "ctx"})
	})
	if out != "ctx" {
		t.Fatalf("got %q", out)
	}
}

type templStub struct{ s string }

func (t templStub) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte(t.s))
	return err
}

type failingPrinter struct {
	err error
}

func (p failingPrinter) Send(Job) error {
	return p.err
}

func TestCursorTempl(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Templ(templStub{s: "tx"})
	})
	if out != "tx" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorTemplCtx(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.TemplCtx(context.Background(), templStub{s: "tcx"})
	})
	if out != "tcx" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyTempl(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any(templStub{s: "any-templ"})
	})
	if out != "any-templ" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyJob(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any(NewJobText(c.Context(), "j"))
	})
	if out != "j" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnyJobSlice(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any([]Job{
			NewJobText(c.Context(), "a"),
			NewJobText(c.Context(), "b"),
		})
	})
	if out != "ab" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorEditor(t *testing.T) {
	ed := EditorFunc(func(c Cursor) error { return c.Text("ed") })
	out := renderElem(t, func(c Cursor) error {
		return c.Editor(ed)
	})
	if out != "ed" {
		t.Fatalf("got %q", out)
	}
}

// --- HeadKind constants -------------------------------------------------

func TestHeadKindDistinct(t *testing.T) {
	if KindContainer == KindRegular || KindRegular == KindVoid {
		t.Fatal("HeadKind not distinct")
	}
}

func TestCursorHeadLifecycleErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(Cursor) error
	}{
		{
			name: "close before submit",
			run: func(c Cursor) error {
				if err := c.Init("div"); err != nil {
					return err
				}
				return c.Close()
			},
		},
		{
			name: "close void after submit",
			run: func(c Cursor) error {
				if err := c.InitVoid("input"); err != nil {
					return err
				}
				if err := c.Submit(); err != nil {
					return err
				}
				return c.Close()
			},
		},
		{
			name: "init while head pending",
			run: func(c Cursor) error {
				if err := c.Init("div"); err != nil {
					return err
				}
				return c.Init("span")
			},
		},
		{
			name: "init void while head pending",
			run: func(c Cursor) error {
				if err := c.Init("div"); err != nil {
					return err
				}
				return c.InitVoid("input")
			},
		},
		{
			name: "init container while head pending",
			run: func(c Cursor) error {
				if err := c.Init("div"); err != nil {
					return err
				}
				return c.InitContainer()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Elem(tc.run).Render(context.Background(), &bytes.Buffer{})
			if err == nil {
				t.Fatal("expected lifecycle error")
			}
		})
	}
}

func TestCursorContentMethodsRequireSubmittedHead(t *testing.T) {
	tests := []struct {
		name string
		run  func(Cursor) error
	}{
		{name: "comp", run: func(c Cursor) error { return c.Comp(compStub{s: "x"}) }},
		{name: "comp ctx", run: func(c Cursor) error { return c.CompCtx(context.Background(), compStub{s: "x"}) }},
		{name: "raw", run: func(c Cursor) error { return c.Raw("x") }},
		{name: "bytes", run: func(c Cursor) error { return c.Bytes([]byte("x")) }},
		{name: "templ", run: func(c Cursor) error { return c.Templ(templStub{s: "x"}) }},
		{name: "templ ctx", run: func(c Cursor) error { return c.TemplCtx(context.Background(), templStub{s: "x"}) }},
		{name: "fprint", run: func(c Cursor) error { return c.Fprint("x") }},
		{name: "many", run: func(c Cursor) error { return c.Many("x") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Elem(func(c Cursor) error {
				if err := c.Init("div"); err != nil {
					return err
				}
				return tc.run(c)
			}).Render(context.Background(), &bytes.Buffer{})
			if err == nil {
				t.Fatal("expected content-state error")
			}
		})
	}
}

func TestCursorAnyCompUsesCompBranch(t *testing.T) {
	out := renderElem(t, func(c Cursor) error {
		return c.Any(compStub{s: "branch"})
	})
	if out != "branch" {
		t.Fatalf("got %q", out)
	}
}

func TestCursorAnySlicesPropagateErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(Cursor) error
	}{
		{
			name: "string slice requires submitted head",
			run: func(c Cursor) error {
				return c.Any([]string{"a"})
			},
		},
		{
			name: "elem slice requires submitted head",
			run: func(c Cursor) error {
				return c.Any([]Elem{
					func(c Cursor) error { return c.Text("a") },
				})
			},
		},
		{
			name: "comp slice requires submitted head",
			run: func(c Cursor) error {
				return c.Any([]Comp{compStub{s: "a"}})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Elem(func(c Cursor) error {
				if err := c.Init("div"); err != nil {
					return err
				}
				return tc.run(c)
			}).Render(context.Background(), &bytes.Buffer{})
			if err == nil {
				t.Fatal("expected slice dispatch error")
			}
		})
	}

	t.Run("job slice propagates printer error", func(t *testing.T) {
		want := errors.New("boom")
		c := NewCursor(context.Background(), failingPrinter{err: want})
		err := c.Any([]Job{NewJobText(c.Context(), "a")})
		if !errors.Is(err, want) {
			t.Fatalf("Any([]Job) error = %v, want %v", err, want)
		}
	})
}
