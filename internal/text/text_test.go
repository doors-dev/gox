package text

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

func TestNewTextEmpty(t *testing.T) {
	tx := NewText()
	if got := tx.String(); got != "" {
		t.Fatalf("String() = %q, want empty", got)
	}
	if got := tx.Cursor(); got.Line() != 0 || got.Column() != 0 {
		t.Fatalf("Cursor() = %v, want 0:0", got)
	}
}

func TestAppendStringSingleLine(t *testing.T) {
	tx := NewText()
	tx.AppendString("hello")
	if got := tx.String(); got != "hello\n" {
		t.Fatalf("String = %q", got)
	}
}

func TestAppendStringMultiline(t *testing.T) {
	tx := NewText()
	tx.AppendString("foo\nbar\nbaz")
	if got := tx.String(); got != "foo\nbar\nbaz\n" {
		t.Fatalf("String = %q", got)
	}
	c := tx.Cursor()
	if c.Line() != 2 || c.Column() != 3 {
		t.Fatalf("Cursor = %v, want 2:3", c)
	}
}

func TestAppendStringTrailingNewline(t *testing.T) {
	tx := NewText()
	tx.AppendString("foo\n")
	if got := tx.String(); got != "foo\n" {
		t.Fatalf("String = %q", got)
	}
	tx.AppendString("bar")
	if got := tx.String(); got != "foo\nbar\n" {
		t.Fatalf("String = %q", got)
	}
}

func TestCRInsertsNewline(t *testing.T) {
	tx := NewText()
	tx.AppendString("foo")
	tx.CR()
	tx.AppendString("bar")
	if got := tx.String(); got != "foo\nbar\n" {
		t.Fatalf("String = %q", got)
	}
}

func TestMustLine(t *testing.T) {
	tx := NewText()
	tx.AppendString("alpha\nbeta\ngamma")
	if got := string(tx.MustLine(1)); got != "beta" {
		t.Fatalf("MustLine(1) = %q", got)
	}
}

func TestSlice(t *testing.T) {
	tx := NewText()
	tx.AppendString("hello world")
	r := common.NewRange(common.NewPos(0, 0), common.NewPos(0, 5))
	if got := string(tx.Slice(r)); got != "hello" {
		t.Fatalf("Slice = %q", got)
	}
}

func TestSliceCursorReturnsNil(t *testing.T) {
	tx := NewText()
	tx.AppendString("hi")
	r := common.NewRange(common.NewPos(0, 1), common.NewPos(0, 1))
	if got := tx.Slice(r); got != nil {
		t.Fatalf("Slice = %v, want nil", got)
	}
}

func TestRune(t *testing.T) {
	tx := NewText()
	tx.AppendString("ab")
	if r, ok := tx.Rune(0); !ok || r != 'a' {
		t.Fatalf("Rune(0) = %q, %v", r, ok)
	}
	if _, ok := tx.Rune(99); ok {
		t.Fatal("Rune(99) ok = true, want false")
	}
	if _, ok := tx.Rune(-1); ok {
		t.Fatal("Rune(-1) ok = true, want false")
	}
}

func TestIndentRefAndCR(t *testing.T) {
	tx := NewText()
	tx.AppendString("\t\tfoo")
	tx.IndentRef([]byte("\t\tfoo"))
	tx.CR()
	tx.AppendString("bar")
	if got := tx.String(); got != "\t\tfoo\n\t\tbar\n" {
		t.Fatalf("String() = %q, want %q", got, "\t\tfoo\n\t\tbar\n")
	}
}

func TestIndentBegEnd(t *testing.T) {
	tx := NewText()
	tx.IndentBeg()
	tx.CR()
	tx.AppendString("body")
	tx.IndentEnd()
	if got := tx.String(); got != "\n\tbody\n" {
		t.Fatalf("String() = %q, want %q", got, "\n\tbody\n")
	}
}

func TestIndentFakeWrapsBraces(t *testing.T) {
	tx := NewText()
	tx.AppendString("head")
	tx.IndentFake()
	tx.CR()
	tx.AppendString("body")
	tx.IndentEnd()
	want := "head\n{\n\tbody\n}\n"
	if got := tx.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestAnnotateOnEmpty(t *testing.T) {
	tx := NewText()
	tx.Annotate("//line foo:1")
	if got := tx.String(); got != "//line foo:1\n" {
		t.Fatalf("String() = %q, want %q", got, "//line foo:1\n")
	}
}

func TestAnnotatePanicMultiline(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx := NewText()
	tx.Annotate("a\nb")
}

func TestAnnotateInsertsBefore(t *testing.T) {
	tx := NewText()
	tx.AppendString("body")
	tx.Annotate("//line foo:1")
	if got := tx.String(); got != "//line foo:1\nbody\n" {
		t.Fatalf("String() = %q, want %q", got, "//line foo:1\nbody\n")
	}
}

func TestClone(t *testing.T) {
	tx := NewText()
	tx.AppendString("hello")
	c := tx.Clone()
	c.AppendString(" world")
	if tx.String() == c.String() {
		t.Fatalf("clone shares state: tx=%q c=%q", tx.String(), c.String())
	}
}

func TestLastPos(t *testing.T) {
	tx := NewText()
	tx.AppendString("a\nb")
	if got := tx.LastPos(); got.Line() < 2 {
		t.Fatalf("LastPos = %v", got)
	}
}

func TestLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("alpha\nbeta\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tx := NewText()
	if err := tx.Load(src); err != nil {
		t.Fatal(err)
	}
	if got := tx.String(); got != "alpha\nbeta\n" {
		t.Fatalf("Load() = %q, want %q", got, "alpha\nbeta\n")
	}
	out := filepath.Join(dir, "out.txt")
	if err := tx.Save(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "alpha\nbeta\n" {
		t.Fatalf("Save() wrote %q, want %q", got, "alpha\nbeta\n")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(p, nil, 0644); err != nil {
		t.Fatal(err)
	}
	tx := NewText()
	if err := tx.Load(p); err != nil {
		t.Fatal(err)
	}
	if tx.String() != "" {
		t.Fatalf("empty Load = %q", tx.String())
	}
}

func TestLoadMissing(t *testing.T) {
	tx := NewText()
	if err := tx.Load("/nonexistent/path/to/file"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateNoop(t *testing.T) {
	tx := NewText()
	tx.AppendString("foo\nbar")
	_, changed, err := tx.Update("foo\nbar")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
}

func TestUpdateChanges(t *testing.T) {
	tx := NewText()
	tx.AppendString("foo\nbar")
	_, changed, err := tx.Update("foo\nbaz")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if got := tx.String(); got != "foo\nbaz\n" {
		t.Fatalf("after Update = %q, want %q", got, "foo\nbaz\n")
	}
}

func TestPatchReplace(t *testing.T) {
	tx := NewText()
	tx.AppendString("hello world")
	r := common.NewRange(common.NewPos(0, 6), common.NewPos(0, 11))
	_, ok, err := tx.Patch(r, "go!")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got := tx.String(); got != "hello go!\n" {
		t.Fatalf("after Patch = %q, want %q", got, "hello go!\n")
	}
}

func TestPatchEndColumnOvershootPreservesTail(t *testing.T) {
	tx := NewText()
	tx.AppendString("aaa\nbbb\nccc")
	r := common.NewRange(common.NewPos(1, 0), common.NewPos(1, 999))
	_, ok, err := tx.Patch(r, "X")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got := tx.String(); got != "aaa\nX\nccc\n" {
		t.Fatalf("after Patch = %q, want %q", got, "aaa\nX\nccc\n")
	}
}

func TestPatchBegColumnOvershootInsertsAtEOL(t *testing.T) {
	tx := NewText()
	tx.AppendString("aaa\nbbb")
	r := common.NewRange(common.NewPos(0, 999), common.NewPos(0, 999))
	_, ok, err := tx.Patch(r, "X")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got := tx.String(); got != "aaaX\nbbb\n" {
		t.Fatalf("after Patch = %q, want %q", got, "aaaX\nbbb\n")
	}
}

func TestPatchBegLinePastEOFColOvershootActsAsColZero(t *testing.T) {
	tx := NewText()
	tx.AppendString("aaa")
	r := common.NewRange(common.NewPos(2, 5), common.NewPos(2, 9))
	_, ok, err := tx.Patch(r, "X")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got := tx.String(); got != "aaa\n\nX\n" {
		t.Fatalf("after Patch = %q, want %q", got, "aaa\n\nX\n")
	}
}

func TestPatchLinePastEOFColZeroAppendsBlankLines(t *testing.T) {
	tx := NewText()
	tx.AppendString("aaa")
	r := common.NewRange(common.NewPos(2, 0), common.NewPos(2, 0))
	_, ok, err := tx.Patch(r, "X")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got := tx.String(); got != "aaa\n\nX\n" {
		t.Fatalf("after Patch = %q, want %q", got, "aaa\n\nX\n")
	}
}

func TestPatchReversedRangeErrors(t *testing.T) {
	tx := NewText()
	tx.AppendString("aaa\nbbb\nccc")
	for _, r := range []common.Range{
		common.NewRange(common.NewPos(2, 1), common.NewPos(0, 1)),
		common.NewRange(common.NewPos(0, 3), common.NewPos(0, 1)),
	} {
		_, ok, err := tx.Patch(r, "X")
		if err == nil {
			t.Fatalf("Patch(%v) error = nil, want error", r)
		}
		if ok {
			t.Fatalf("Patch(%v) ok = true, want false", r)
		}
	}
	if got := tx.String(); got != "aaa\nbbb\nccc\n" {
		t.Fatalf("text corrupted = %q", got)
	}
}

func TestPatchNegativePositionsClamped(t *testing.T) {
	tx := NewText()
	tx.AppendString("aaa\nbbb")
	r := common.NewRange(common.NewPos(-3, -7), common.NewPos(-1, -1))
	_, ok, err := tx.Patch(r, "X")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got := tx.String(); got != "Xaaa\nbbb\n" {
		t.Fatalf("after Patch = %q, want %q", got, "Xaaa\nbbb\n")
	}
}

func TestPatchColumnAtLineLengthBoundary(t *testing.T) {
	tx := NewText()
	tx.AppendString("abc\ndef")
	r := common.NewRange(common.NewPos(0, 3), common.NewPos(0, 3))
	_, ok, err := tx.Patch(r, "X")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got := tx.String(); got != "abcX\ndef\n" {
		t.Fatalf("after insert Patch = %q, want %q", got, "abcX\ndef\n")
	}
	tx2 := NewText()
	tx2.AppendString("abc\ndef")
	r2 := common.NewRange(common.NewPos(0, 0), common.NewPos(0, 3))
	_, ok, err = tx2.Patch(r2, "X")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got := tx2.String(); got != "X\ndef\n" {
		t.Fatalf("after replace Patch = %q, want %q", got, "X\ndef\n")
	}
}

func TestIntoFromPosNegativeLine(t *testing.T) {
	tx := NewText()
	tx.AppendString("a")
	p := common.NewPos(-1, 3)
	if got := tx.IntoPos(common.UTF16, p); got != p {
		t.Fatalf("IntoPos negative line = %v, want %v", got, p)
	}
	if got := tx.FromPos(common.UTF16, p); got != p {
		t.Fatalf("FromPos negative line = %v, want %v", got, p)
	}
}

func TestUtf16to8And8to16ASCII(t *testing.T) {
	b := []byte("hello")
	if got := Utf16to8(b, 3); got != 3 {
		t.Errorf("Utf16to8 = %d", got)
	}
	if got := Utf8to16(b, 3); got != 3 {
		t.Errorf("Utf8to16 = %d", got)
	}
}

func TestUtf16to8MultiByte(t *testing.T) {
	b := []byte("héllo")
	if got := Utf16to8(b, 2); got != 3 {
		t.Errorf("Utf16to8 = %d, want 3", got)
	}
	if got := Utf8to16(b, 3); got != 2 {
		t.Errorf("Utf8to16 = %d, want 2", got)
	}
}

func TestUtf16to8Surrogate(t *testing.T) {
	b := []byte("a😀b")
	if got := Utf16to8(b, 3); got != 5 {
		t.Errorf("Utf16to8 surrogate = %d, want 5", got)
	}
	if got := Utf8to16(b, 5); got != 3 {
		t.Errorf("Utf8to16 surrogate = %d, want 3", got)
	}
}

func TestIntoFromPosUTF8Identity(t *testing.T) {
	tx := NewText()
	tx.AppendString("abc")
	p := common.NewPos(0, 2)
	if got := tx.IntoPos(common.UTF8, p); got.Column() != 2 {
		t.Fatalf("IntoPos UTF8 = %v", got)
	}
	if got := tx.FromPos(common.UTF8, p); got.Column() != 2 {
		t.Fatalf("FromPos UTF8 = %v", got)
	}
}

func TestIntoFromPosUTF16Conversion(t *testing.T) {
	tx := NewText()
	tx.AppendString("héllo")
	p16 := common.NewPos(0, 2)
	got := tx.IntoPos(common.UTF16, p16)
	if got.Column() != 3 {
		t.Fatalf("IntoPos UTF16 = %v, want 0:3", got)
	}
	p8 := common.NewPos(0, 3)
	got2 := tx.FromPos(common.UTF16, p8)
	if got2.Column() != 2 {
		t.Fatalf("FromPos UTF16 = %v, want 0:2", got2)
	}
}

func TestIntoRangeFromRange(t *testing.T) {
	tx := NewText()
	tx.AppendString("héllo world")
	r := common.NewRange(common.NewPos(0, 0), common.NewPos(0, 2))
	got := tx.IntoRange(common.UTF16, r)
	if got.End().Column() != 3 {
		t.Fatalf("IntoRange end col = %d, want 3", got.End().Column())
	}
	got2 := tx.FromRange(common.UTF16, common.NewRange(common.NewPos(0, 0), common.NewPos(0, 3)))
	if got2.End().Column() != 2 {
		t.Fatalf("FromRange end col = %d, want 2", got2.End().Column())
	}
}

func TestIntoFromPosOutOfRange(t *testing.T) {
	tx := NewText()
	tx.AppendString("a")
	p := common.NewPos(99, 5)
	if got := tx.IntoPos(common.UTF16, p); got.Line() != 99 || got.Column() != 5 {
		t.Fatalf("IntoPos out-of-range = %v", got)
	}
	if got := tx.FromPos(common.UTF16, p); got.Line() != 99 || got.Column() != 5 {
		t.Fatalf("FromPos out-of-range = %v", got)
	}
}

func TestOffsetArithmetic(t *testing.T) {
	o := newOffset(2, 5)
	if o.Beg() != 2 || o.End() != 5 || o.Len() != 3 {
		t.Fatalf("offset = %v", o)
	}
	o.Shift(1)
	if o.Beg() != 3 || o.End() != 6 {
		t.Fatalf("after Shift = %v", o)
	}
	o.ExpandLeft(2)
	if o.Beg() != 1 {
		t.Fatalf("after ExpandLeft = %v", o)
	}
	o.ExpandRight(4)
	if o.End() != 10 {
		t.Fatalf("after ExpandRight = %v", o)
	}
	o.SetEnd(7)
	if o.End() != 7 {
		t.Fatalf("after SetEnd = %v", o)
	}
}

func TestLoadNewlineVariants(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"empty", "", ""},
		{"single newline", "\n", ""},
		{"only newlines", "\n\n\n", ""},
		{"no trailing newline", "a", "a\n"},
		{"trailing newline", "a\n", "a\n"},
		{"leading newline", "\na", "\na\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "f.gox")
			if err := os.WriteFile(p, []byte(c.content), 0644); err != nil {
				t.Fatal(err)
			}
			tx := NewText()
			if err := tx.Load(p); err != nil {
				t.Fatal(err)
			}
			if got := tx.String(); got != c.want {
				t.Fatalf("Load(%q) = %q, want %q", c.content, got, c.want)
			}
		})
	}
}
