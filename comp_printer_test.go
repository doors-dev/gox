package gox

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestElemNilPrintAndRender(t *testing.T) {
	var e Elem
	printBuf := &bytes.Buffer{}
	if err := e.Print(context.Background(), NewPrinter(printBuf)); err != nil {
		t.Fatalf("Print(nil) error = %v", err)
	}
	if printBuf.Len() != 0 {
		t.Fatalf("Print(nil) wrote %q, want empty output", printBuf.String())
	}
	renderBuf := &bytes.Buffer{}
	if err := e.Render(context.Background(), renderBuf); err != nil {
		t.Fatalf("Render(nil) error = %v", err)
	}
	if renderBuf.Len() != 0 {
		t.Fatalf("Render(nil) wrote %q, want empty output", renderBuf.String())
	}
}

func TestElemPrintExpandsOntoPrinter(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("print"), "root")
	e := Elem(func(c Cursor) error {
		if err := c.Init("span"); err != nil {
			return err
		}
		if err := c.Submit(); err != nil {
			return err
		}
		if err := c.Text("ok"); err != nil {
			return err
		}
		return c.Close()
	})

	var kinds []string
	buf := &bytes.Buffer{}
	printer := PrinterFunc(func(j Job) error {
		if j.Context() != ctx {
			t.Fatal("job context did not round-trip the provided context")
		}
		switch j.(type) {
		case *JobOpen:
			kinds = append(kinds, "open")
		case *JobText:
			kinds = append(kinds, "text")
		case *JobClose:
			kinds = append(kinds, "close")
		default:
			kinds = append(kinds, "other")
		}
		return j.Output(buf)
	})
	if err := e.Print(ctx, printer); err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	want := []string{"open", "text", "close"}
	if len(kinds) != len(want) {
		t.Fatalf("Print() sent %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("Print() sent %v, want %v", kinds, want)
		}
	}
	if buf.String() != "<span>ok</span>" {
		t.Fatalf("Print() wrote %q, want %q", buf.String(), "<span>ok</span>")
	}
}

func TestElemPrintDefaultPrinterMatchesRender(t *testing.T) {
	e := Elem(func(c Cursor) error {
		if err := c.Init("span"); err != nil {
			return err
		}
		if err := c.Set("class", "badge"); err != nil {
			return err
		}
		if err := c.Submit(); err != nil {
			return err
		}
		if err := c.Text("a&b"); err != nil {
			return err
		}
		return c.Close()
	})

	printBuf := &bytes.Buffer{}
	if err := e.Print(context.Background(), NewPrinter(printBuf)); err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	renderBuf := &bytes.Buffer{}
	if err := e.Render(context.Background(), renderBuf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if printBuf.String() != renderBuf.String() {
		t.Fatalf("Print() output = %q, Render() output = %q", printBuf.String(), renderBuf.String())
	}
}

func TestPrinterSendCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := NewPrinter(&bytes.Buffer{})
	err := p.Send(NewJobText(ctx, "ignored"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send(canceled) error = %v, want context.Canceled", err)
	}
}
