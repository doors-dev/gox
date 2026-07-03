package workspace

import (
	"go/parser"
	"go/token"
	"testing"
)

func mustParseGo(t *testing.T, out string) {
	t.Helper()
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "tpl.x.go", out, 0); err != nil {
		t.Errorf("generated target does not parse: %v\n--- output ---\n%s", err, out)
	}
}

func TestElemParamTypeInDeclaration(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Page(cb elem(x int)) {
	<p>hi</p>
}
`
	out := assemble(t, src)
	mustContain(t, out, "func Page(cb func(x int) gox.Elem) gox.Elem {")
	mustNotContain(t, out, "elem(")
	mustParseGo(t, out)
}

func TestElemParamTypeInLiteral(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

var g = elem(cb elem(x int)) {
	<p>hi</p>
}
`
	out := assemble(t, src)
	mustContain(t, out, "var g = func(cb func(x int) gox.Elem) gox.Elem {")
	mustNotContain(t, out, "elem(")
	mustParseGo(t, out)
}

func TestElemParamTypeInTypeAlias(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

type R = elem(f elem(x int))
`
	out := assemble(t, src)
	mustContain(t, out, "type R = func(f func(x int) gox.Elem) gox.Elem")
	mustNotContain(t, out, "elem(")
	mustParseGo(t, out)
}

func TestElemParamTypeNested(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Page(cb elem(f elem(x int))) {
	<p>hi</p>
}
`
	out := assemble(t, src)
	mustContain(t, out, "func Page(cb func(f func(x int) gox.Elem) gox.Elem) gox.Elem {")
	mustNotContain(t, out, "elem(")
	mustParseGo(t, out)
}

func TestElemParamTypeMixedParams(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem F(a int, cb elem(), b string) {
	<p>hi</p>
}
`
	out := assemble(t, src)
	mustContain(t, out, "func F(a int, cb func() gox.Elem, b string) gox.Elem {")
	mustNotContain(t, out, "elem(")
	mustParseGo(t, out)
}

func TestElemParamTypeMethodReceiver(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

type App struct{}

elem (a App) Main(cb elem()) {
	<p>hi</p>
}
`
	out := assemble(t, src)
	mustContain(t, out, "func (a App) Main(cb func() gox.Elem) gox.Elem")
	mustNotContain(t, out, "elem(")
	mustParseGo(t, out)
}

func TestElemParamTypeGenerics(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem List[T any](r elem(item T)) {
	<p>hi</p>
}
`
	out := assemble(t, src)
	mustContain(t, out, "func List[T any](r func(item T) gox.Elem) gox.Elem")
	mustNotContain(t, out, "elem(")
	mustParseGo(t, out)
}

func TestElemParamTypePlainGoDoubleNested(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

func F(cb elem(f elem(x int))) {}
`
	out := assemble(t, src)
	mustContain(t, out, "func F(cb func(f func(x int) gox.Elem) gox.Elem) {}")
	mustNotContain(t, out, "elem(")
	mustParseGo(t, out)
}
