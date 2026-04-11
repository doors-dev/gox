package translator

import (
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

func r(bl, bc, el, ec int) common.Range {
	return common.NewRange(common.NewPos(bl, bc), common.NewPos(el, ec))
}

func TestPortalConvertSameLineShift(t *testing.T) {
	// source line 0 cols 0..5  →  target line 10 cols 4..9 (shift +10 lines, +4 cols)
	p := Portal{self: r(0, 0, 0, 5), other: r(10, 4, 10, 9)}
	// pos at start of source maps to start of target
	got := p.Convert(common.NewPos(0, 0))
	if got.Line() != 10 || got.Column() != 4 {
		t.Fatalf("Convert begin = %v", got)
	}
	// pos at column 3 within source line maps to col 7 in target
	got = p.Convert(common.NewPos(0, 3))
	if got.Line() != 10 || got.Column() != 7 {
		t.Fatalf("Convert mid = %v", got)
	}
	// pos at end maps to end
	got = p.Convert(common.NewPos(0, 5))
	if got.Line() != 10 || got.Column() != 9 {
		t.Fatalf("Convert end = %v", got)
	}
}

func TestPortalReverse(t *testing.T) {
	p := Portal{self: r(0, 0, 0, 5), other: r(10, 4, 10, 9)}
	rev := p.Reverse()
	if rev.Self().Beg().Line() != 10 || rev.Other().Beg().Line() != 0 {
		t.Fatalf("Reverse swapped wrong: %v", rev)
	}
}

func TestPortalSelfOther(t *testing.T) {
	p := Portal{self: r(1, 2, 3, 4), other: r(5, 6, 7, 8)}
	if p.Self().Beg().Line() != 1 || p.Other().Beg().Line() != 5 {
		t.Fatal("Self/Other accessors")
	}
	_ = p.String()
}

// --- Translator round-trip ----------------------------------------------

func TestTranslatorRoundTrip(t *testing.T) {
	tr := NewTranslator()
	// source 0:0..0:5  ↔  target 10:0..10:5
	tr.Append(r(0, 0, 0, 5), r(10, 0, 10, 5))

	// inside the source range
	got, ok := tr.ConvertToTarget(common.NewPos(0, 2), false)
	if !ok || got.Line() != 10 || got.Column() != 2 {
		t.Fatalf("ConvertToTarget = %v ok=%v", got, ok)
	}
	got, ok = tr.ConvertToSource(common.NewPos(10, 2), false)
	if !ok || got.Line() != 0 || got.Column() != 2 {
		t.Fatalf("ConvertToSource = %v ok=%v", got, ok)
	}
}

func TestTranslatorOutOfRange(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(0, 0, 0, 5), r(10, 0, 10, 5))
	if _, ok := tr.ConvertToTarget(common.NewPos(5, 0), false); ok {
		t.Fatal("expected miss for out-of-range source")
	}
}

func TestTranslatorRangeConvert(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(0, 0, 0, 10), r(20, 0, 20, 10))
	got, ok := tr.RangeConvertToTarget(r(0, 2, 0, 6))
	if !ok || got.Beg().Line() != 20 || got.End().Column() != 6 {
		t.Fatalf("RangeConvertToTarget = %v ok=%v", got, ok)
	}
	got, ok = tr.RangeConvertToSource(r(20, 2, 20, 6))
	if !ok || got.Beg().Line() != 0 || got.End().Column() != 6 {
		t.Fatalf("RangeConvertToSource = %v ok=%v", got, ok)
	}
}

func TestTranslatorRangeOutOfPortal(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(0, 0, 0, 10), r(20, 0, 20, 10))
	// End is past the portal
	if _, ok := tr.RangeConvertToTarget(r(0, 2, 5, 6)); ok {
		t.Fatal("expected RangeConvertToTarget miss")
	}
	if _, ok := tr.RangeConvertToSource(r(20, 2, 25, 6)); ok {
		t.Fatal("expected RangeConvertToSource miss")
	}
}

func TestTranslatorApproximateNoPortals(t *testing.T) {
	tr := NewTranslator()
	if _, ok := tr.ApproximateConvertToTarget(common.NewPos(1, 1)); ok {
		t.Fatal("expected miss")
	}
	if _, ok := tr.ApproximateConvertToSource(common.NewPos(1, 1)); ok {
		t.Fatal("expected miss")
	}
}

func TestTranslatorApproximate(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(0, 0, 0, 5), r(10, 0, 10, 5))
	tr.Append(r(2, 0, 2, 5), r(12, 0, 12, 5))
	// Pos between the two portals — closest line decides.
	got, ok := tr.ApproximateConvertToTarget(common.NewPos(1, 0))
	if !ok {
		t.Fatal("approximate miss")
	}
	if got.Line() < 10 || got.Line() > 12 {
		t.Fatalf("approximate = %v", got)
	}
}

func TestTranslatorAppendOverlapPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on overlap")
		}
	}()
	tr := NewTranslator()
	tr.Append(r(0, 0, 5, 0), r(10, 0, 15, 0))
	tr.Append(r(2, 0, 6, 0), r(12, 0, 16, 0))
}

func TestPortalsFindPosBeforeAll(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(5, 0, 5, 5), r(15, 0, 15, 5))
	if _, ok := tr.SourcePos(common.NewPos(0, 0), false); ok {
		t.Fatal("expected miss before any portal")
	}
}

func TestPortalsFindPosExactBeg(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(0, 0, 0, 5), r(10, 0, 10, 5))
	if _, ok := tr.SourcePos(common.NewPos(0, 0), false); !ok {
		t.Fatal("expected hit at portal start")
	}
}

func TestApproximateClosestColumn(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(0, 0, 0, 2), r(10, 0, 10, 2))
	tr.Append(r(0, 8, 0, 10), r(10, 8, 10, 10))
	// pos at col 5 — equidistant by line, closer column to right
	got, ok := tr.ApproximateConvertToTarget(common.NewPos(0, 5))
	if !ok {
		t.Fatal("approximate miss")
	}
	_ = got
}

func TestApproximateAfterAll(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(0, 0, 0, 5), r(10, 0, 10, 5))
	if _, ok := tr.ApproximateConvertToTarget(common.NewPos(99, 0)); !ok {
		t.Fatal("approximate after-all should still pick the only portal")
	}
}

func TestApproximateBeforeAll(t *testing.T) {
	tr := NewTranslator()
	tr.Append(r(5, 0, 5, 5), r(10, 0, 10, 5))
	if _, ok := tr.ApproximateConvertToTarget(common.NewPos(0, 0)); !ok {
		t.Fatal("approximate before-all should pick the only portal")
	}
}

func TestPortalConvertEnd(t *testing.T) {
	// pos == self.End() exactly should map to other.End()
	p := Portal{self: r(0, 0, 0, 5), other: r(10, 4, 10, 9)}
	got := p.Convert(common.NewPos(0, 5))
	if got.Line() != 10 || got.Column() != 9 {
		t.Fatalf("Convert end = %v", got)
	}
}

func TestPortalConvertAfterEnd(t *testing.T) {
	// pos > self.End() — should still produce a position based on end shift
	p := Portal{self: r(0, 0, 2, 5), other: r(10, 0, 12, 5)}
	got := p.Convert(common.NewPos(3, 0))
	// shifted by +10 lines
	if got.Line() != 13 {
		t.Fatalf("Convert after end = %v", got)
	}
}
