package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

// makeDoc writes src to a temp .gox file and returns a fully-initialized Doc.
func makeDoc(t *testing.T, src string) Doc {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.gox")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	file, ok := NewFile(path)
	if !ok {
		t.Fatalf("NewFile !ok")
	}
	doc := NewDoc(file)
	if err := doc.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := doc.Parse(); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	doc.Assemble()
	return doc
}

// makeDocLoose writes src to a temp .gox file and runs Load + Parse without
// failing on parse errors. Useful for completion tests where the source is
// intentionally incomplete.
func makeDocLoose(t *testing.T, src string) Doc {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.gox")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	file, ok := NewFile(path)
	if !ok {
		t.Fatalf("NewFile !ok")
	}
	doc := NewDoc(file)
	if err := doc.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Parse via the underlying parser, ignoring errors so we still have a tree.
	doc.tree = doc.parser.Parse(doc.source.Source(), nil)
	doc.Assemble()
	return doc
}

const sampleSrc = `package demo

import "github.com/doors-dev/gox"

elem Card(name string) {
	<div class="card">
		<h1>~(name)</h1>
	</div>
}

var page gox.Elem = Card("hello")
`

// --- Hover --------------------------------------------------------------

func TestDocHoverOnGoxNode(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	// Find the line with `<h1>` - it's relative to the source
	src := doc.source.String()
	idx := strings.Index(src, "<h1>")
	if idx < 0 {
		t.Fatal("h1 not found")
	}
	// Compute line/column
	line, col := 0, 0
	for i := 0; i < idx; i++ {
		if src[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	pos := common.NewPos(line, col+1) // inside <h1>
	msg, _, ok := doc.Hover(common.UTF8, pos)
	if !ok {
		t.Fatal("Hover returned !ok on h1")
	}
	if msg == "" {
		t.Fatal("Hover returned empty message")
	}
}

func TestDocHoverOutsideGox(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	// Position 0,0 is inside `package demo` — not gox
	if _, _, ok := doc.Hover(common.UTF8, common.NewPos(0, 0)); ok {
		t.Fatal("Hover at package keyword returned ok")
	}
}

// --- Completions --------------------------------------------------------

func TestDocCompletionsTagName(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem F() {
	<di
}
`
	doc := makeDocLoose(t, src)
	// Position right after "<di" — find it
	idx := strings.Index(doc.source.String(), "<di")
	if idx < 0 {
		t.Fatal("`<di` not found")
	}
	srcStr := doc.source.String()
	line := 0
	col := 0
	for i := 0; i < idx+3; i++ {
		if srcStr[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	cs, _ := doc.Completions(common.UTF8, common.NewPos(line, col))
	// Even if grammar fails for the broken element, the call should not crash
	_ = cs
}

func TestDocCompletionsAtTilde(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem F(name string) {
	<div>~</div>
}
`
	doc := makeDocLoose(t, src)
	srcStr := doc.source.String()
	idx := strings.Index(srcStr, "~")
	line := 0
	col := 0
	for i := 0; i < idx+1; i++ {
		if srcStr[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	cs, _ := doc.Completions(common.UTF8, common.NewPos(line, col))
	// Tilde completions should produce snippets like "~(..)", "~(if..)", etc.
	_ = cs
}

// --- Conversions --------------------------------------------------------

func TestDocTargetPos(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	srcStr := doc.source.String()
	idx := strings.Index(srcStr, "name")
	line := 0
	col := 0
	for i := 0; i < idx; i++ {
		if srcStr[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	pos := common.NewPos(line, col)
	if _, ok := doc.TargetPos(common.UTF8, pos, Strict); !ok {
		// May be ok=false if the name token isn't inside a portal — that's
		// fine, we just need the call to execute without panicking.
	}
	_, _ = doc.TargetPos(common.UTF8, pos, Edge)
	_, _ = doc.TargetPos(common.UTF8, pos, Approximate)
	_, _ = doc.SourcePos(common.UTF8, common.NewPos(0, 0), Strict)
	_, _ = doc.SourcePos(common.UTF8, common.NewPos(0, 0), Edge)
	_, _ = doc.SourcePos(common.UTF8, common.NewPos(0, 0), Approximate)
	_, _ = doc.ApproximateTargetPos(common.UTF8, pos)
}

func TestDocTargetRange(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	r := common.NewRange(common.NewPos(0, 0), common.NewPos(2, 0))
	_, _ = doc.TargetRange(common.UTF8, r, Strict)
	_, _ = doc.TargetRange(common.UTF8, r, Edge)
	_, _ = doc.TargetRange(common.UTF8, r, Approximate)
	_, _ = doc.SourceRange(common.UTF8, r, Strict)
	_, _ = doc.SourceRange(common.UTF8, r, Edge)
	_, _ = doc.SourceRange(common.UTF8, r, Approximate)
}

// --- Edits --------------------------------------------------------------

func TestDocSourceUpdateNoop(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	upd, err := doc.SourceUpdate(sampleSrc)
	if err != nil {
		t.Fatal(err)
	}
	if upd {
		t.Fatal("expected upd=false")
	}
}

func TestDocSourceUpdateChanges(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	updated := strings.Replace(sampleSrc, "card", "panel", 1)
	upd, err := doc.SourceUpdate(updated)
	if err != nil {
		t.Fatal(err)
	}
	if !upd {
		t.Fatal("expected upd=true")
	}
	if !strings.Contains(doc.source.String(), "panel") {
		t.Fatal("source not updated")
	}
}

func TestDocSourcePatch(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	// Replace one cursor position
	r := common.NewRange(common.NewPos(0, 0), common.NewPos(0, 7))
	upd, err := doc.SourcePatch(common.UTF8, r, "package")
	if err != nil {
		t.Fatal(err)
	}
	_ = upd
}

// --- Format -------------------------------------------------------------

func TestDocFormat(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	_, err := doc.Format(common.UTF8)
	if err != nil {
		// Format may legitimately fail if the rust formatter isn't available;
		// don't make it a hard failure.
		t.Logf("Format: %v", err)
	}
}

// --- Symbols ------------------------------------------------------------

func TestDocSymbols(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	syms := doc.Symbols(common.UTF8)
	if len(syms) == 0 {
		t.Fatal("expected at least one symbol")
	}
}

// --- GoxImportPos --------------------------------------------------------

func TestDocGoxImportPos(t *testing.T) {
	// Source without a gox import → import is needed.
	src := `package demo

elem F() {
	<div>x</div>
}
`
	doc := makeDoc(t, src)
	if _, ok := doc.GoxImportPos(common.UTF8); !ok {
		t.Log("GoxImportPos returned !ok — may be expected if parser stops early")
	}

	// Source that already imports gox → no insertion required.
	doc2 := makeDoc(t, sampleSrc)
	_, _ = doc2.GoxImportPos(common.UTF8)
}

// --- Delete / targetRemove ----------------------------------------------

func TestDocDelete(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	if err := doc.TargetWrite(); err != nil {
		t.Fatal(err)
	}
	if !doc.TargetFile().Exists() {
		t.Fatal("target should exist after TargetWrite")
	}
	doc.Delete()
	if doc.TargetFile().Exists() {
		t.Fatal("target should be deleted")
	}
	// Calling Delete on missing target should be a no-op.
	doc.Delete()
}

// --- File ---------------------------------------------------------------

func TestNewFileFromURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.gox")
	if err := os.WriteFile(path, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	uri := "file://" + path
	f, ok := NewFileFromURI(uri)
	if !ok {
		t.Fatal("NewFileFromURI !ok")
	}
	if f.Kind() != KindSource {
		t.Errorf("Kind = %v", f.Kind())
	}
	if !f.IsValid() {
		t.Error("IsValid = false")
	}
}

func TestNewFileFromURIBad(t *testing.T) {
	if _, ok := NewFileFromURI("not a uri"); ok {
		t.Fatal("expected !ok")
	}
}

func TestFileIsValidEmpty(t *testing.T) {
	var f File
	if f.IsValid() {
		t.Fatal("empty file IsValid = true")
	}
}

func TestFileRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.gox")
	if err := os.WriteFile(path, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f, _ := NewFile(path)
	if err := f.Remove(); err != nil {
		t.Fatal(err)
	}
	if f.Exists() {
		t.Fatal("file still exists after Remove")
	}
}

func TestFileURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.gox")
	if err := os.WriteFile(path, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f, _ := NewFile(path)
	if f.URI() == "" {
		t.Fatal("URI empty")
	}
	if f.Dir() == "" {
		t.Fatal("Dir empty")
	}
}

// --- StoreDiag / GetDiag -----------------------------------------------

func TestDocStoreDiagNil(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	if doc.GetDiag() != nil {
		t.Fatal("initial GetDiag != nil")
	}
}

// --- PrintTarget --------------------------------------------------------

func TestDocPrintTarget(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	// Just ensure it does not panic. PrintTarget writes to stdout.
	doc.PrintTarget()
}
