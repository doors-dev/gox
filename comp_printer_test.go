package gox

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestElemNilPrintAndRender(t *testing.T) {
	var e Elem
	if err := e.Print(context.Background(), NewPrinter(&bytes.Buffer{})); err != nil {
		t.Fatalf("Print(nil) error = %v", err)
	}
	if err := e.Render(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatalf("Render(nil) error = %v", err)
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
