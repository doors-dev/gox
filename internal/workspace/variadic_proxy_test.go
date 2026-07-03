package workspace

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

func assembleDoc(t *testing.T, src string) (Doc, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.gox")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	file, ok := NewFile(path)
	if !ok {
		t.Fatalf("NewFile(%q) returned !ok", path)
	}
	doc := NewDoc(file)
	t.Cleanup(doc.Close)
	if err := doc.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	parseErr := doc.Parse()
	doc.Assemble()
	return doc, parseErr
}

func mustParseGoTarget(t *testing.T, out string) {
	t.Helper()
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "tpl.x.go", out, 0); err != nil {
		t.Errorf("target is not valid Go: %v\n--- target ---\n%s", err, out)
	}
}

func mustHaveNeedle(t *testing.T, out string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(out, n) {
			t.Errorf("output missing %q\n--- output ---\n%s", n, out)
		}
	}
}

func mustLackNeedle(t *testing.T, out string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(out, n) {
			t.Errorf("output unexpectedly contained %q\n--- output ---\n%s", n, out)
		}
	}
}

func TestProxyVariadicArgRejected(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(ps []any) {
	<>
		~>(ps...) <div>x</div>
	</>
}
`
	doc, parseErr := assembleDoc(t, src)
	if parseErr == nil {
		t.Fatal("Parse: expected error, got nil")
	}
	if !strings.Contains(parseErr.Error(), variadicProxyMessage) {
		t.Errorf("Parse error missing %q, got: %v", variadicProxyMessage, parseErr)
	}
	errs := doc.SyntaxErrors(common.UTF8)
	if len(errs) != 1 {
		t.Fatalf("SyntaxErrors: expected 1 error, got %d: %+v", len(errs), errs)
	}
	if errs[0].Message != variadicProxyMessage {
		t.Errorf("SyntaxErrors message = %q, want %q", errs[0].Message, variadicProxyMessage)
	}
	want := common.NewRange(common.NewPos(6, 5), common.NewPos(6, 10))
	if errs[0].Range != want {
		t.Errorf("SyntaxErrors range = %v, want %v", errs[0].Range, want)
	}
	out := doc.TargetContent()
	mustParseGoTarget(t, out)
	mustLackNeedle(t, out, "ps...).Proxy")
}

func TestProxyVariadicArgMixedRejected(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(a any, ps []any) {
	<>
		~>(a, ps...) <div>x</div>
	</>
}
`
	doc, parseErr := assembleDoc(t, src)
	if parseErr == nil {
		t.Fatal("Parse: expected error, got nil")
	}
	errs := doc.SyntaxErrors(common.UTF8)
	if len(errs) != 1 {
		t.Fatalf("SyntaxErrors: expected 1 error, got %d: %+v", len(errs), errs)
	}
	if errs[0].Message != variadicProxyMessage {
		t.Errorf("SyntaxErrors message = %q, want %q", errs[0].Message, variadicProxyMessage)
	}
	want := common.NewRange(common.NewPos(6, 8), common.NewPos(6, 13))
	if errs[0].Range != want {
		t.Errorf("SyntaxErrors range = %v, want %v", errs[0].Range, want)
	}
	out := doc.TargetContent()
	mustParseGoTarget(t, out)
	mustHaveNeedle(t, out, "(a).Proxy(__c, gox.Elem(func(__c gox.Cursor)")
	mustLackNeedle(t, out, "ps...).Proxy")
}

func TestProxySingleArgAccepted(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(p any) {
	<>
		~>(p) <div>x</div>
	</>
}
`
	doc, parseErr := assembleDoc(t, src)
	if parseErr != nil {
		t.Fatalf("Parse: %v", parseErr)
	}
	if errs := doc.SyntaxErrors(common.UTF8); len(errs) != 0 {
		t.Errorf("SyntaxErrors: expected none, got %+v", errs)
	}
	out := doc.TargetContent()
	mustParseGoTarget(t, out)
	mustHaveNeedle(t, out, "(p).Proxy(__c, gox.Elem(func(__c gox.Cursor)")
}

func TestProxyMultiArgAccepted(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(a, b any) {
	<>
		~>(a, b) <div>x</div>
	</>
}
`
	doc, parseErr := assembleDoc(t, src)
	if parseErr != nil {
		t.Fatalf("Parse: %v", parseErr)
	}
	if errs := doc.SyntaxErrors(common.UTF8); len(errs) != 0 {
		t.Errorf("SyntaxErrors: expected none, got %+v", errs)
	}
	out := doc.TargetContent()
	mustParseGoTarget(t, out)
	mustHaveNeedle(t, out,
		"(a).Proxy(__c, gox.Elem(func(__c gox.Cursor)",
		"(b).Proxy(__c, gox.Elem(func(__c gox.Cursor)",
	)
}

func TestPlaceholderVariadicArgAccepted(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(ps []any) {
	<div>~(ps...)</div>
}
`
	doc, parseErr := assembleDoc(t, src)
	if parseErr != nil {
		t.Fatalf("Parse: %v", parseErr)
	}
	if errs := doc.SyntaxErrors(common.UTF8); len(errs) != 0 {
		t.Errorf("SyntaxErrors: expected none, got %+v", errs)
	}
	out := doc.TargetContent()
	mustParseGoTarget(t, out)
	mustHaveNeedle(t, out, "__e = __c.Many(ps...)")
}
