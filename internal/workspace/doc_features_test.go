package workspace

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

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

func TestDocHoverOnGoxNode(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	src := doc.source.String()
	idx := strings.Index(src, "<h1>")
	if idx < 0 {
		t.Fatal("h1 not found")
	}
	line, col := 0, 0
	for i := 0; i < idx; i++ {
		if src[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	pos := common.NewPos(line, col+1)
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
	if _, _, ok := doc.Hover(common.UTF8, common.NewPos(0, 0)); ok {
		t.Fatal("Hover at package keyword returned ok")
	}
}

func TestDocCompletionsTagName(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem F() {
	<di
}
`
	doc := makeDocLoose(t, src)
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
	if !hasCompletionLabel(cs, "<div/>") {
		t.Fatalf("tag completions = %#v, want <div/>", cs)
	}
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
	if !hasCompletionLabel(cs, "~(..)") {
		t.Fatalf("tilde completions = %#v, want ~(..)", cs)
	}
	if !hasCompletionLabel(cs, "~(if..)") {
		t.Fatalf("tilde completions = %#v, want ~(if..)", cs)
	}
}

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
	_, _ = doc.TargetPos(common.UTF8, pos, Strict)
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
	r := common.NewRange(common.NewPos(0, 0), common.NewPos(0, 7))
	upd, err := doc.SourcePatch(common.UTF8, r, "package")
	if err != nil {
		t.Fatal(err)
	}
	if !upd {
		t.Fatal("expected upd=true for non-empty patch range")
	}
	if got := doc.source.String(); got != sampleSrc {
		t.Fatalf("source after no-op patch = %q, want %q", got, sampleSrc)
	}
}

func TestDocFormat(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	_, err := doc.Format(common.UTF8)
	if err != nil {
		t.Logf("Format: %v", err)
	}
}

func TestDocSymbols(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	syms := doc.Symbols(common.UTF8)
	if len(syms) == 0 {
		t.Fatal("expected at least one symbol")
	}
}

func TestDocGoxImportPos(t *testing.T) {
	src := `package demo

elem F() {
	<div>x</div>
}
`
	doc := makeDoc(t, src)
	if _, ok := doc.GoxImportPos(common.UTF8); !ok {
		t.Fatal("GoxImportPos() = !ok, want insertion point")
	}

	doc2 := makeDoc(t, sampleSrc)
	if _, ok := doc2.GoxImportPos(common.UTF8); ok {
		t.Fatal("GoxImportPos() = ok for existing import, want !ok")
	}
}

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
	doc.Delete()
}

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

func TestDocStoreDiagNil(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	if doc.GetDiag() != nil {
		t.Fatal("initial GetDiag != nil")
	}
}

func TestDocPrintTarget(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})
	doc.PrintTarget()
	if err := w.Close(); err != nil {
		t.Fatalf("Close(write pipe) error = %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	if got := string(out); got != doc.TargetContent() {
		t.Fatalf("PrintTarget() = %q, want %q", got, doc.TargetContent())
	}
}

func TestSourcePatchColumnOvershootDoesNotPoison(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	ran := common.NewRange(common.NewPos(10, 0), common.NewPos(10, 999))
	upd, err := doc.SourcePatch(common.UTF8, ran, `var page gox.Elem = Card("bye")`)
	if err != nil {
		t.Fatalf("SourcePatch() error = %v", err)
	}
	if !upd {
		t.Fatal("SourcePatch() upd = false, want true")
	}
	if doc.Err() != nil {
		t.Fatalf("doc.Err() = %v, want nil", doc.Err())
	}
	src := doc.source.String()
	if !strings.Contains(src, `Card("bye")`) {
		t.Fatalf("source missing patched content: %q", src)
	}
	if strings.Contains(src, `Card("hello")`) {
		t.Fatalf("source still has old content: %q", src)
	}
}

func TestSourcePatchReversedRangePoisons(t *testing.T) {
	doc := makeDoc(t, sampleSrc)
	ran := common.NewRange(common.NewPos(10, 5), common.NewPos(4, 0))
	_, err := doc.SourcePatch(common.UTF8, ran, "x")
	if err == nil {
		t.Fatal("SourcePatch() error = nil, want error")
	}
	if doc.Err() == nil {
		t.Fatal("doc.Err() = nil, want poisoned doc")
	}
}
