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

func TestElemPrintSendsRootJobComp(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("print"), "root")
	ran := false
	e := Elem(func(c Cursor) error {
		ran = true
		return c.Text("ok")
	})

	var sent Job
	printer := PrinterFunc(func(j Job) error {
		sent = j
		return nil
	})
	if err := e.Print(ctx, printer); err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	if ran {
		t.Fatal("Print() rendered Elem directly, want root JobComp sent to printer")
	}
	job, ok := sent.(*JobComp)
	if !ok {
		t.Fatalf("Print() sent %T, want *JobComp", sent)
	}
	if job.Context() != ctx {
		t.Fatal("JobComp context did not round-trip the provided context")
	}

	buf := &bytes.Buffer{}
	if err := job.Output(buf); err != nil {
		t.Fatalf("JobComp.Output() error = %v", err)
	}
	if !ran {
		t.Fatal("JobComp.Output() did not render Elem")
	}
	if buf.String() != "ok" {
		t.Fatalf("JobComp.Output() wrote %q, want ok", buf.String())
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
