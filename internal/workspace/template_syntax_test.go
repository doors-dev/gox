package workspace

// Tests covering the template syntax features documented in
// /Users/alex/Lib/doors/docs/03-template-syntax.md.
//
// Each test writes a .gox source to a temporary directory, runs the
// Doc lifecycle (Load -> Parse -> Assemble), and asserts that the
// generated .x.go target contains the expected gox.Cursor calls.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assemble writes src to a .gox file inside t.TempDir() and returns the
// generated target text. It fails the test on parse errors so individual
// tests can focus on output assertions.
func assemble(t *testing.T, src string) string {
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
	if err := doc.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := doc.Parse(); err != nil {
		t.Fatalf("Parse: %v\nsource:\n%s", err, src)
	}
	doc.Assemble()
	return doc.TargetContent()
}

// mustContain asserts that out contains every needle. Useful for checking
// that the assembler emitted the expected cursor operations without
// pinning the exact whitespace/format.
func mustContain(t *testing.T, out string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(out, n) {
			t.Errorf("output missing %q\n--- output ---\n%s", n, out)
		}
	}
}

func mustNotContain(t *testing.T, out string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(out, n) {
			t.Errorf("output unexpectedly contained %q\n--- output ---\n%s", n, out)
		}
	}
}

// --- HTML as expression ---------------------------------------------------

// Doc section: "HTML as Expression". A bare <h1>...</h1> assigned to a
// gox.Elem variable should compile to a gox.Elem(func(__c gox.Cursor) ...).
func TestHTMLAsExpression(t *testing.T) {
	src := `package demo

import "github.com/doors-dev/gox"

var hello gox.Elem = <h1>Hello World!</h1>
`
	out := assemble(t, src)
	mustContain(t, out,
		"var hello gox.Elem = gox.Elem(func(__c gox.Cursor)",
		`__e = __c.Init("h1")`,
		`__e = __c.Text("Hello World!")`,
		"__e = __c.Close()",
	)
}

// Doc section: "HTML as Expression". The fragment form `<>...</>` should
// not emit any tags but should still wrap children in an InitContainer call.
func TestFragmentEmitsInitContainer(t *testing.T) {
	src := `package demo

import "github.com/doors-dev/gox"

var frag gox.Elem = <>
	<h1>Header</h1>
	<p>Text</p>
</>
`
	out := assemble(t, src)
	mustContain(t, out,
		"__e = __c.InitContainer()",
		`__e = __c.Init("h1")`,
		`__e = __c.Init("p")`,
	)
	// A fragment must not produce <> tags or an Init("") call.
	mustNotContain(t, out, `__c.Init("")`, `__c.Init("<>")`)
}

// --- elem keyword ----------------------------------------------------------

// Doc section: "Elem Primitive". `elem header() { ... }` is sugar for
// `func header() gox.Elem { return ... }`.
func TestElemKeywordFunction(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem header() {
	<h1>Header</h1>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"func header() gox.Elem {",
		"return gox.Elem(func(__c gox.Cursor)",
		`__e = __c.Init("h1")`,
		`__e = __c.Text("Header")`,
	)
}

// Doc section: "Elem Primitive". elem methods on a receiver should expand
// to `func (a App) Main() gox.Elem`.
func TestElemKeywordMethod(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

type App struct{}

elem (a App) Main() {
	<div>app</div>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"func (a App) Main() gox.Elem {",
		`__e = __c.Init("div")`,
	)
}

// Doc section: "Elem Primitive". Anonymous elem expressions assigned to a
// variable should expand to `func() gox.Elem { return ... }`.
func TestElemKeywordAnonymous(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

var f = elem() {
	<h1>Header</h1>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"var f = func() gox.Elem",
		"return gox.Elem(func(__c gox.Cursor)",
	)
}

// --- Placeholder ~(...) ----------------------------------------------------

// Doc section: "Placeholder". `~(name)` should emit `__c.Any(name)`.
func TestPlaceholderSingleArg(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Card(name string, price int) {
	<card>
		<header>~(name)</header>
		<span>~(price)</span>
	</card>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"__e = __c.Any(name)",
		"__e = __c.Any(price)",
	)
}

// Doc section: "Placeholder". A placeholder with multiple comma-separated
// arguments should be lowered to `__c.Many(...)`.
func TestPlaceholderMultiArg(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Hello(name string, surname string) {
	Hello ~(name, " ", surname)!
}
`
	out := assemble(t, src)
	mustContain(t, out,
		`__e = __c.Many(name, " ", surname)`,
	)
}

// Doc section: "Placeholder". String literals can omit the parentheses
// (`~"hello"`). They should still be wrapped in `__c.Any(...)`.
func TestPlaceholderImplicitStringLiteral(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Greeting() {
	<p>~"hello world"</p>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		`__e = __c.Any("hello world")`,
	)
}

// Doc section: "Placeholder". Composite literals (struct/slice/map) can
// omit the parentheses too: `~item{...}`.
func TestPlaceholderImplicitCompositeLiteral(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

type item struct {
	Name string
}

elem (i item) Main() {
	~(i.Name)
}

elem Items() {
	<div>
		~item{Name: "Product A"}
	</div>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		`__e = __c.Any(item{Name: "Product A"})`,
	)
}

// --- Conditions and loops --------------------------------------------------

// Doc section: "Conditions and Loops". `~(if cond { ... })` should emit a
// raw Go `if` block whose body contains the cursor calls for the children.
func TestConditionalIf(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Greeting(loggedIn bool) {
	<div>
		~(if loggedIn {
			Hello!
		} else {
			Please log in
		})
	</div>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"if loggedIn {",
		`__e = __c.Text("Hello!")`,
		"} else  {",
		`__e = __c.Text("Please log in")`,
	)
}

// Doc section: "Conditions and Loops". `~(for ... { ... })` should emit a
// raw Go `for` loop wrapping the children's cursor calls.
func TestForLoop(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

type user struct {
	Name  string
	Email string
}

elem Table(users []user) {
	<table>
		~(for _, u := range users {
			<tr>
				<td>~(u.Name)</td>
				<td>~(u.Email)</td>
			</tr>
		})
	</table>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"for _, u := range users {",
		`__e = __c.Init("tr")`,
		`__e = __c.Init("td")`,
		"__e = __c.Any(u.Name)",
		"__e = __c.Any(u.Email)",
	)
}

// --- Attributes ------------------------------------------------------------

// Doc section: "Attributes". A literal attribute is set via Set and a
// parenthesised Go expression is passed through verbatim.
func TestAttributesLiteralAndExpression(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem block(id string) {
	<div id=(id) class="card">Content</div>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		`__e = __c.Set("id", id)`,
		`__e = __c.Set("class", "card")`,
	)
}

// Doc section: "Attributes". A boolean attribute without a value (`<input
// disabled>`) should be set to `true`.
func TestAttributesBooleanShorthand(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Field() {
	<input disabled>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		`__e = __c.Set("disabled", true)`,
	)
}

// Doc section: "Modifiers". `<button (LandingAction)>` attaches an attribute
// modifier via `__c.Modify(LandingAction)`.
func TestAttributeModifier(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(LandingAction any) {
	<button (LandingAction)>Request Demo!</button>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"__e = __c.Modify(LandingAction)",
		`__e = __c.Init("button")`,
		`__e = __c.Text("Request Demo!")`,
	)
}

// --- Code: inline expression and Go snippets -------------------------------

// Doc section: "GoX expression". `~({ ... })` should be lowered to an
// IIFE returning `any` so the result can be passed to `__c.Any`.
func TestInlineExpression(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(id int) {
	<div>
		~func {
			return id
		}
	</div>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"__e = __c.Any(func() any",
		"return id",
		"}())",
	)
}

// Doc section: "Go snippet". `~~ ... ~~` switches into raw Go mode; the
// snippet body should appear verbatim inside the rendered function.
func TestGoSnippet(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo() {
	~{
		x := 42
		_ = x
	}
	<span>~(x)</span>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"x := 42",
		"_ = x",
		"__e = __c.Any(x)",
	)
}

// --- Comments --------------------------------------------------------------

// Doc section: "Comments". Tilde-prefixed comments (`~// ...`, `~/* ... */`)
// are template-only — they should be stripped entirely from the generated
// file and never appear as Text/Any/Raw cursor calls. HTML comments, on the
// other hand, are part of the rendered document and should be emitted as Raw.
func TestComments(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo() {
	<div>
		~// hidden line comment
		~/* hidden
		   multiline */
		<!-- visible html comment -->
		<span>x</span>
	</div>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		`__e = __c.Raw("<!-- visible html comment -->")`,
		`__e = __c.Init("span")`,
	)
	// Tilde comments are template-level and must not leak into the
	// generated code in any form.
	mustNotContain(t, out,
		"hidden line comment",
		"hidden\n",
		"multiline",
	)
}

// --- Element proxy ---------------------------------------------------------

// Doc section: "Element Proxy". `~>(p) <div>...</div>` wraps the following
// element subtree in `(p).Proxy(__c, gox.Elem(func(__c gox.Cursor) ...))`.
func TestElementProxySingle(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(proxy any) {
	<>
		~>(proxy) <div>
			Proxy can apply transformations to this HTML.
		</div>
	</>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"= (proxy).Proxy(__c, gox.Elem(func(__c gox.Cursor)",
		`__e = __c.Init("div")`,
	)
}

// Doc section: "Element Proxy". A comma-separated list of proxies should
// open a Proxy call for each one, nested in declaration order.
func TestElementProxyMultiple(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Demo(Track, Transform any) {
	<>
		~>(Track, Transform) <div>x</div>
	</>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		"(Track).Proxy(__c, gox.Elem(func(__c gox.Cursor)",
		"(Transform).Proxy(__c, gox.Elem(func(__c gox.Cursor)",
		`__e = __c.Init("div")`,
	)
}

// --- Raw <:> ... </:> ------------------------------------------------------

// Doc section: "Raw". Content wrapped in `<:>...</:>` should be emitted via
// `__c.Raw(...)` as a single string literal, with no per-tag Init calls.
func TestRawTag(t *testing.T) {
	src := `package demo

import _ "github.com/doors-dev/gox"

elem Icon() {
	<svg viewBox="0 0 24 24">
		<:>
			<path d="M1 2"/>
			<path d="M3 4"/>
		</:>
	</svg>
}
`
	out := assemble(t, src)
	mustContain(t, out,
		`__e = __c.Init("svg")`,
		`__e = __c.Set("viewBox", "0 0 24 24")`,
		`__e = __c.Raw(`,
		`<path d=\"M1 2\"/>`,
		`<path d=\"M3 4\"/>`,
	)
	// The <path> elements inside <:> must not be turned into Init calls.
	mustNotContain(t, out, `__c.Init("path")`)
}
