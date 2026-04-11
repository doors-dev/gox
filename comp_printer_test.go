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

func TestPrinterSendCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := NewPrinter(&bytes.Buffer{})
	err := p.Send(NewJobText(ctx, "ignored"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send(canceled) error = %v, want context.Canceled", err)
	}
}
