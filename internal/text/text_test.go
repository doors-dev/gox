package text

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

// --- NewText / basic state ----------------------------------------------

func TestNewTextEmpty(t *testing.T) {
	tx := NewText()
	if got := tx.String(); got != "" {
		t.Fatalf("String() = %q, want empty", got)
	}
	if got := tx.Cursor(); got.Line() != 0 || got.Column() != 0 {
		t.Fatalf("Cursor() = %v, want 0:0", got)
	}
}

// --- Append / AppendString / CR -----------------------------------------

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
	// Cursor reports last line / column
	c := tx.Cursor()
	if c.Line() != 2 || c.Column() != 3 {
		t.Fatalf("Cursor = %v, want 2:3", c)
	}
}

func TestAppendStringTrailingNewline(t *testing.T) {
	tx := NewText()
	tx.AppendString("foo\n")
	// After "foo\n" we should have a final empty line for the next append.
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

// --- MustLine / Slice / Rune -------------------------------------------

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

// --- Indents -----------------------------------------------------------

func TestIndentRefAndCR(t *testing.T) {
	tx := NewText()
	tx.AppendString("\t\tfoo")
	tx.IndentRef([]byte("\t\tfoo"))
	tx.CR()
	tx.AppendString("bar")
	// New line should inherit the "\t\t" prefix from IndentRef.
	got := tx.String()
	if !strings.Contains(got, "\n\t\tbar") {
		t.Fatalf("String = %q", got)
	}
}

func TestIndentBegEnd(t *testing.T) {
	tx := NewText()
	tx.IndentBeg()
	tx.CR()
	tx.AppendString("body")
	tx.IndentEnd()
	got := tx.String()
	if !strings.Contains(got, "\n\tbody") {
		t.Fatalf("String = %q", got)
	}
}

func TestIndentFakeWrapsBraces(t *testing.T) {
	tx := NewText()
	tx.AppendString("head")
	tx.IndentFake()
	tx.CR()
	tx.AppendString("body")
	tx.IndentEnd()
	got := tx.String()
	if !strings.Contains(got, "{") || !strings.Contains(got, "}") {
		t.Fatalf("Fake indent missing braces: %q", got)
	}
	if !strings.Contains(got, "\tbody") {
		t.Fatalf("Fake indent body not indented: %q", got)
	}
}

// --- Annotate ----------------------------------------------------------

func TestAnnotateOnEmpty(t *testing.T) {
	tx := NewText()
	tx.Annotate("//line foo:1")
	if !strings.Contains(tx.String(), "//line foo:1") {
		t.Fatalf("String = %q", tx.String())
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
	got := tx.String()
	if !strings.Contains(got, "//line foo:1") || !strings.Contains(got, "body") {
		t.Fatalf("String = %q", got)
	}
	if strings.Index(got, "//line") > strings.Index(got, "body") {
		t.Fatalf("annotation not before body: %q", got)
	}
}

// --- Clone -------------------------------------------------------------

func TestClone(t *testing.T) {
	tx := NewText()
	tx.AppendString("hello")
	c := tx.Clone()
	c.AppendString(" world")
	if tx.String() == c.String() {
		t.Fatalf("clone shares state: tx=%q c=%q", tx.String(), c.String())
	}
}

// --- LastPos -----------------------------------------------------------

func TestLastPos(t *testing.T) {
	tx := NewText()
	tx.AppendString("a\nb")
	if got := tx.LastPos(); got.Line() < 2 {
		t.Fatalf("LastPos = %v", got)
	}
}

// --- Load / Save -------------------------------------------------------

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
	if got := tx.String(); !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Fatalf("Load got %q", got)
	}
	out := filepath.Join(dir, "out.txt")
	if err := tx.Save(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "alpha") {
		t.Fatalf("Save wrote %q", b)
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

// --- Update / Patch ----------------------------------------------------

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
	if !strings.Contains(tx.String(), "baz") {
		t.Fatalf("after Update = %q", tx.String())
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
	if got := tx.String(); !strings.Contains(got, "hello go!") {
		t.Fatalf("after Patch = %q", got)
	}
}

// --- Encoding conversions ----------------------------------------------

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
	// "héllo" — 'é' is 2 bytes in UTF-8 but 1 unit in UTF-16.
	b := []byte("héllo")
	// utf16 col 2 → after 'h' + 'é' → byte offset 3
	if got := Utf16to8(b, 2); got != 3 {
		t.Errorf("Utf16to8 = %d, want 3", got)
	}
	if got := Utf8to16(b, 3); got != 2 {
		t.Errorf("Utf8to16 = %d, want 2", got)
	}
}

func TestUtf16to8Surrogate(t *testing.T) {
	// 😀 is 4 bytes utf-8 and 2 utf-16 units.
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
	// UTF-16 col 2 → UTF-8 col 3
	p16 := common.NewPos(0, 2)
	got := tx.IntoPos(common.UTF16, p16)
	if got.Column() != 3 {
		t.Fatalf("IntoPos UTF16 = %v, want 0:3", got)
	}
	// Reverse
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
	// pos beyond all lines is returned as-is.
	p := common.NewPos(99, 5)
	if got := tx.IntoPos(common.UTF16, p); got.Line() != 99 || got.Column() != 5 {
		t.Fatalf("IntoPos out-of-range = %v", got)
	}
	if got := tx.FromPos(common.UTF16, p); got.Line() != 99 || got.Column() != 5 {
		t.Fatalf("FromPos out-of-range = %v", got)
	}
}

// --- offset arithmetic --------------------------------------------------

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
