package common

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestNoPosIsInvalid(t *testing.T) {
	p := NoPos()
	if p.IsValid() {
		t.Fatalf("NoPos().IsValid() = true, want false")
	}
}

func TestNewPosAccessors(t *testing.T) {
	p := NewPos(3, 7)
	if p.Line() != 3 || p.Column() != 7 {
		t.Fatalf("NewPos(3,7) = (%d,%d), want (3,7)", p.Line(), p.Column())
	}
	if !p.IsValid() {
		t.Fatalf("NewPos(3,7).IsValid() = false, want true")
	}
	if got := p.String(); got != "3:7" {
		t.Fatalf("String() = %q, want %q", got, "3:7")
	}
}

func TestNewTSPosRoundTrip(t *testing.T) {
	tp := tree_sitter.Point{Row: 4, Column: 9}
	p := NewTSPos(tp)
	if p.Line() != 4 || p.Column() != 9 {
		t.Fatalf("NewTSPos = (%d,%d), want (4,9)", p.Line(), p.Column())
	}
	back := p.TS()
	if back.Row != 4 || back.Column != 9 {
		t.Fatalf("TS() = %+v, want Row=4 Column=9", back)
	}
}

func TestPosCompare(t *testing.T) {
	cases := []struct {
		a, b Pos
		want int
	}{
		{NewPos(1, 1), NewPos(1, 1), 0},
		{NewPos(1, 1), NewPos(2, 0), -1},
		{NewPos(2, 0), NewPos(1, 1), 1},
		{NewPos(1, 1), NewPos(1, 2), -1},
		{NewPos(1, 3), NewPos(1, 2), 1},
	}
	for _, c := range cases {
		if got := c.a.Compare(c.b); got != c.want {
			t.Errorf("%v.Compare(%v) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestPosAdvance(t *testing.T) {
	p := NewPos(2, 5).Advance()
	if p.Line() != 2 || p.Column() != 6 {
		t.Fatalf("Advance() = %v, want 2:6", p)
	}
}

func TestNoRangeIsInvalid(t *testing.T) {
	r := NoRange()
	if r.IsValid() {
		t.Fatalf("NoRange().IsValid() = true, want false")
	}
}

func TestRangeBasics(t *testing.T) {
	r := NewRange(NewPos(1, 2), NewPos(3, 4))
	if !r.IsValid() {
		t.Fatalf("IsValid() = false, want true")
	}
	if r.Beg().Line() != 1 || r.Beg().Column() != 2 {
		t.Fatalf("Beg = %v, want 1:2", r.Beg())
	}
	if r.End().Line() != 3 || r.End().Column() != 4 {
		t.Fatalf("End = %v, want 3:4", r.End())
	}
	if r.CountLines() != 3 {
		t.Fatalf("CountLines = %d, want 3", r.CountLines())
	}
	if got := r.String(); got != "1:2-3:4" {
		t.Fatalf("String = %q, want %q", got, "1:2-3:4")
	}
	if r.IsCursor() {
		t.Fatalf("IsCursor = true, want false")
	}
}

func TestRangeIsCursor(t *testing.T) {
	r := NewRange(NewPos(2, 5), NewPos(2, 5))
	if !r.IsCursor() {
		t.Fatalf("IsCursor = false, want true")
	}
}

func TestRangeLines(t *testing.T) {
	r := NewRange(NewPos(2, 0), NewPos(5, 0))
	got := r.Lines()
	want := []int{2, 3, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("Lines() len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Lines()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestRangeContains(t *testing.T) {
	r := NewRange(NewPos(2, 3), NewPos(4, 7))

	type tc struct {
		name       string
		pos        Pos
		includeEnd bool
		want       bool
	}
	cases := []tc{
		{"before-line", NewPos(1, 0), false, false},
		{"after-line", NewPos(5, 0), false, false},
		{"before-col-on-beg-line", NewPos(2, 2), false, false},
		{"at-beg", NewPos(2, 3), false, true},
		{"middle-line", NewPos(3, 100), false, true},
		{"on-end-line-before-end", NewPos(4, 6), false, true},
		{"on-end-line-at-end-exclusive", NewPos(4, 7), false, false},
		{"on-end-line-at-end-inclusive", NewPos(4, 7), true, true},
		{"on-end-line-after-end", NewPos(4, 8), true, false},
	}
	for _, c := range cases {
		if got := r.Contains(c.pos, c.includeEnd); got != c.want {
			t.Errorf("%s: Contains(%v, %v) = %v, want %v", c.name, c.pos, c.includeEnd, got, c.want)
		}
	}
}

func TestNewTSRange(t *testing.T) {
	tsr := tree_sitter.Range{
		StartPoint: tree_sitter.Point{Row: 1, Column: 2},
		EndPoint:   tree_sitter.Point{Row: 3, Column: 4},
	}
	r := NewTSRange(tsr)
	if r.Beg().Line() != 1 || r.Beg().Column() != 2 || r.End().Line() != 3 || r.End().Column() != 4 {
		t.Fatalf("NewTSRange = %v", r)
	}
	back := r.TS()
	if back.StartPoint.Row != 1 || back.EndPoint.Row != 3 {
		t.Fatalf("TS roundtrip = %+v", back)
	}
}

func TestBytesNoCopy(t *testing.T) {
	s := "hello"
	b := Bytes(s)
	if string(b) != s {
		t.Fatalf("Bytes() = %q, want %q", string(b), s)
	}
	if len(b) != len(s) {
		t.Fatalf("len = %d, want %d", len(b), len(s))
	}
}

func TestErrWrapper(t *testing.T) {
	err := NewErr(nil, "boom")
	if err.Error() != "boom" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "boom")
	}
	if err.Wire != nil {
		t.Fatalf("Wire = %v, want nil", err.Wire)
	}
}

type fakeErr struct{}

func (fakeErr) Error() string { return "wrapped" }

func TestFromErr(t *testing.T) {
	e := FromErr(nil, fakeErr{})
	if e.Error() != "wrapped" {
		t.Fatalf("Error() = %q, want %q", e.Error(), "wrapped")
	}
}
