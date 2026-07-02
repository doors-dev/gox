package workspace

import (
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

const actionsSrc = `package demo

import "github.com/doors-dev/gox"

elem Card(name string, price int) {
	~~ label := "$" ~~
	<span>~(label)~(price * 2)</span>
}

var page gox.Elem = Card("hello", 10)
`

func cursorAt(t *testing.T, d Doc, marker string, offsetInto int) common.Range {
	t.Helper()
	src := d.source.String()
	idx := strings.Index(src, marker)
	if idx < 0 {
		t.Fatalf("marker %q not found", marker)
	}
	idx += offsetInto
	line, col := 0, 0
	for i := 0; i < idx; i++ {
		if src[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	p := common.NewPos(line, col)
	return common.NewRange(p, p)
}

func applyEdits(t *testing.T, src string, edits []ActionEdit) string {
	t.Helper()
	type off struct {
		beg, end int
		text     string
	}
	offs := make([]off, 0, len(edits))
	for _, e := range edits {
		offs = append(offs, off{
			beg:  byteOffset(src, e.Range.Beg().Line(), e.Range.Beg().Column()),
			end:  byteOffset(src, e.Range.End().Line(), e.Range.End().Column()),
			text: e.NewText,
		})
	}
	for i := 0; i < len(offs); i++ {
		for j := i + 1; j < len(offs); j++ {
			if offs[j].beg > offs[i].beg {
				offs[i], offs[j] = offs[j], offs[i]
			}
		}
	}
	out := src
	for _, o := range offs {
		out = out[:o.beg] + o.text + out[o.end:]
	}
	return out
}

func byteOffset(src string, line, col int) int {
	i, l := 0, 0
	for l < line && i < len(src) {
		if src[i] == '\n' {
			l++
		}
		i++
	}
	return i + col
}

func reparse(t *testing.T, src string) string {
	t.Helper()
	d := makeDoc(t, src)
	return d.TargetContent()
}

func findAction(actions []CodeAction, kind string) (CodeAction, bool) {
	for _, a := range actions {
		if a.Kind == kind {
			return a, true
		}
	}
	return CodeAction{}, false
}

func TestExtractVariableInTemplate(t *testing.T) {
	d := makeDoc(t, actionsSrc)
	ran := cursorAt(t, d, "price * 2", 2)
	actions := d.ExtractActions(common.UTF8, ran)

	a, ok := findAction(actions, KindExtractVariable)
	if !ok {
		t.Fatalf("no extract variable action; got %d actions", len(actions))
	}
	if len(a.Edits) != 2 {
		t.Fatalf("expected 2 edits, got %d", len(a.Edits))
	}
	var insert, replace *ActionEdit
	for i := range a.Edits {
		if a.Edits[i].Range.IsCursor() {
			insert = &a.Edits[i]
		} else {
			replace = &a.Edits[i]
		}
	}
	if insert == nil || replace == nil {
		t.Fatalf("expected one insertion and one replacement, got %+v", a.Edits)
	}
	if replace.NewText != "newVar" {
		t.Fatalf("replacement newText = %q, want newVar", replace.NewText)
	}
	if !strings.Contains(insert.NewText, "~~ newVar := price * 2 ~~") {
		t.Fatalf("insertion should wrap binding in a snippet, got %q", insert.NewText)
	}
}

func TestExtractVariableInGoSnippet(t *testing.T) {
	d := makeDoc(t, actionsSrc)
	ran := cursorAt(t, d, `label := "$"`, 10)
	actions := d.ExtractActions(common.UTF8, ran)

	a, ok := findAction(actions, KindExtractVariable)
	if !ok {
		t.Fatalf("no extract variable action in go snippet")
	}
	for _, e := range a.Edits {
		if e.Range.IsCursor() {
			if strings.Contains(e.NewText, "~~") {
				t.Fatalf("go-snippet insertion must not re-wrap in ~~, got %q", e.NewText)
			}
			if !strings.Contains(e.NewText, "newVar := ") {
				t.Fatalf("expected raw go binding, got %q", e.NewText)
			}
		}
	}
}

func TestExtractConstantOnlyForConstExpr(t *testing.T) {
	d := makeDoc(t, actionsSrc)
	ran := cursorAt(t, d, "price * 2", 2)
	actions := d.ExtractActions(common.UTF8, ran)
	if _, ok := findAction(actions, KindExtractConstant); ok {
		t.Fatalf("must not offer extract constant for non-constant expression")
	}

	ran2 := cursorAt(t, d, `label := "$"`, 10)
	actions2 := d.ExtractActions(common.UTF8, ran2)
	a, ok := findAction(actions2, KindExtractConstant)
	if !ok {
		t.Fatalf("expected extract constant for string literal")
	}
	foundConst := false
	for _, e := range a.Edits {
		if e.Range.IsCursor() && strings.Contains(e.NewText, "const newConst = ") {
			foundConst = true
		}
	}
	if !foundConst {
		t.Fatalf("constant binding not found in edits: %+v", a.Edits)
	}
}

func TestExtractTopLevelVariable(t *testing.T) {
	d := makeDoc(t, actionsSrc)
	ran := cursorAt(t, d, `Card("hello", 10)`, 2)
	actions := d.ExtractActions(common.UTF8, ran)
	a, ok := findAction(actions, KindExtractVariable)
	if !ok {
		t.Fatalf("no extract variable action at top level")
	}
	foundVar := false
	for _, e := range a.Edits {
		if e.Range.IsCursor() && strings.HasPrefix(e.NewText, "var newVar = Card(") {
			foundVar = true
		}
	}
	if !foundVar {
		t.Fatalf("expected top-level `var newVar = ...`, edits: %+v", a.Edits)
	}
}

func TestExtractToNewFile(t *testing.T) {
	d := makeDoc(t, actionsSrc)
	ran := cursorAt(t, d, "elem Card", 6)
	actions := d.ExtractActions(common.UTF8, ran)
	a, ok := findAction(actions, KindExtractToNewFile)
	if !ok {
		t.Fatalf("no extract-to-new-file action; got %d actions", len(actions))
	}
	if a.CreateFile == nil {
		t.Fatalf("expected a CreateFile op")
	}
	if !strings.HasSuffix(a.CreateFile.URI, "card.gox") {
		t.Fatalf("new file uri = %q, want suffix card.gox", a.CreateFile.URI)
	}
	c := a.CreateFile.Text
	if !strings.HasPrefix(c, "package demo\n") {
		t.Fatalf("new file must start with package clause, got:\n%s", c)
	}
	if !strings.Contains(c, "elem Card(name string, price int) {") {
		t.Fatalf("new file must contain the verbatim decl, got:\n%s", c)
	}
	if len(a.Edits) != 1 || a.Edits[0].NewText != "" {
		t.Fatalf("expected a single deletion edit, got %+v", a.Edits)
	}
	remaining := applyEdits(t, actionsSrc, a.Edits)
	if strings.Contains(remaining, "elem Card") {
		t.Fatalf("declaration not removed from original:\n%s", remaining)
	}
}

func TestExtractToNewFileOnlyOnHeader(t *testing.T) {
	d := makeDoc(t, actionsSrc)

	onName := cursorAt(t, d, "elem Card", 6)
	if _, ok := findAction(d.ExtractActions(common.UTF8, onName), KindExtractToNewFile); !ok {
		t.Fatal("toNewFile must be offered on the declaration name")
	}

	onKeyword := cursorAt(t, d, "elem Card", 1)
	if _, ok := findAction(d.ExtractActions(common.UTF8, onKeyword), KindExtractToNewFile); !ok {
		t.Fatal("toNewFile must be offered on the elem/func keyword")
	}

	inBody := cursorAt(t, d, "<span>~(label)", 2)
	if _, ok := findAction(d.ExtractActions(common.UTF8, inBody), KindExtractToNewFile); ok {
		t.Fatal("toNewFile must not be offered from inside the declaration body")
	}

	inSignatureAfterName := cursorAt(t, d, "name string", 1)
	if _, ok := findAction(d.ExtractActions(common.UTF8, inSignatureAfterName), KindExtractToNewFile); ok {
		t.Fatal("toNewFile must not be offered from the parameter list")
	}
}

const componentSrc = `package demo

import "github.com/doors-dev/gox"

elem Card(name string) {
	<div class="card">
		<header>Static Title</header>
		<span>~(name)</span>
	</div>
}

func widget() gox.Elem {
	return <aside>Sidebar</aside>
}
`

func TestExtractComponentStaticMarkup(t *testing.T) {
	d := makeDoc(t, componentSrc)
	ran := cursorAt(t, d, "Static Title", 2)
	actions := d.ExtractActions(common.UTF8, ran)
	a, ok := findAction(actions, KindExtractElem)
	if !ok {
		t.Fatalf("no extract-to-elem action for static markup; got %d actions", len(actions))
	}
	var insert, replace *ActionEdit
	for i := range a.Edits {
		if a.Edits[i].Range.IsCursor() {
			insert = &a.Edits[i]
		} else {
			replace = &a.Edits[i]
		}
	}
	if insert == nil || replace == nil {
		t.Fatalf("expected insert + replace edits, got %+v", a.Edits)
	}
	if replace.NewText != "~(NewElem())" {
		t.Fatalf("template-position replacement = %q, want ~(NewElem())", replace.NewText)
	}
	if !strings.Contains(insert.NewText, "elem NewElem() {") ||
		!strings.Contains(insert.NewText, "<header>Static Title</header>") {
		t.Fatalf("new elem text wrong: %q", insert.NewText)
	}
	result := applyEdits(t, componentSrc, a.Edits)
	target := reparse(t, result)
	if !strings.Contains(target, "NewElem") {
		t.Fatalf("generated target missing NewElem:\n%s", result)
	}
}

func TestExtractComponentSkipsCaptures(t *testing.T) {
	d := makeDoc(t, componentSrc)
	ran := cursorAt(t, d, "~(name)", 2)
	actions := d.ExtractActions(common.UTF8, ran)
	if _, ok := findAction(actions, KindExtractElem); ok {
		t.Fatalf("must not offer extract-to-elem for markup that captures a local")
	}
}

func TestExtractComponentExpressionPosition(t *testing.T) {
	d := makeDoc(t, componentSrc)
	ran := cursorAt(t, d, "<aside>Sidebar", 2)
	actions := d.ExtractActions(common.UTF8, ran)
	a, ok := findAction(actions, KindExtractElem)
	if !ok {
		t.Fatalf("no extract-to-elem action in expression position")
	}
	for _, e := range a.Edits {
		if !e.Range.IsCursor() && e.NewText != "NewElem()" {
			t.Fatalf("expression-position replacement = %q, want NewElem()", e.NewText)
		}
	}
	result := applyEdits(t, componentSrc, a.Edits)
	reparse(t, result)
}

func TestExtractVariableSkipsDeclarationNameAndStatement(t *testing.T) {
	src := `package demo

import "github.com/doors-dev/gox"

elem Card() {
	~~ active := compute() ~~
	~~ compute() ~~
	<span>~(active)</span>
}

func compute() string { return "x" }
`
	d := makeDoc(t, src)

	onName := cursorAt(t, d, "active := compute", 2)
	if _, ok := findAction(d.ExtractActions(common.UTF8, onName), KindExtractVariable); ok {
		t.Fatalf("must not offer extract variable on a declaration name")
	}

	onStatement := cursorAt(t, d, "~~ compute() ~~", 5)
	if _, ok := findAction(d.ExtractActions(common.UTF8, onStatement), KindExtractVariable); ok {
		t.Fatalf("must not offer extract variable on a whole expression statement")
	}

	onRHS := cursorAt(t, d, "active := compute()", len("active := comp"))
	a, ok := findAction(d.ExtractActions(common.UTF8, onRHS), KindExtractVariable)
	if !ok {
		t.Fatalf("expected extract variable on the := right-hand side")
	}
	result := applyEdits(t, src, a.Edits)
	if !strings.Contains(result, "newVar := compute()") || !strings.Contains(result, "active := newVar") {
		t.Fatalf("extraction corrupted the binding:\n%s", result)
	}
	reparse(t, result)
}

func TestExtractResilientToErrorsElsewhere(t *testing.T) {
	src := "package demo\n\nimport \"github.com/doors-dev/gox\"\n\nfunc Widget(price int) gox.Elem {\n\treturn <span>~(price)</span>\n}\n\nelem Broken( {\n\t<div></div>\n}\n"
	d := makeDocLoose(t, src)
	if !d.tree.RootNode().HasError() {
		t.Skip("grammar did not flag the broken construct; test is moot")
	}
	ran := cursorAt(t, d, "func Widget", 6)
	if _, ok := findAction(d.ExtractActions(common.UTF8, ran), KindExtractToNewFile); !ok {
		t.Fatalf("toNewFile must still be offered on a clean decl when the grammar errors elsewhere")
	}
}

func TestExtractVariableInlinePlacement(t *testing.T) {
	src := "package demo\n\nelem Card(price int) {\n\t<span>Total: ~(price * 2) USD</span>\n}\n"
	d := makeDoc(t, src)
	ran := cursorAt(t, d, "price * 2", 2)
	a, ok := findAction(d.ExtractActions(common.UTF8, ran), KindExtractVariable)
	if !ok {
		t.Fatal("no extract variable action")
	}
	result := applyEdits(t, src, a.Edits)
	if !strings.Contains(result, "<span>Total: ~(newVar) USD</span>") {
		t.Fatalf("inline text/HTML was corrupted by the binding insertion:\n%s", result)
	}
	if !strings.Contains(result, "~~ newVar := price * 2 ~~\n") {
		t.Fatalf("binding should be on its own line:\n%s", result)
	}
	reparse(t, result)
}

func TestExtractToNewFileGroupedDecl(t *testing.T) {
	src := "package demo\n\nvar (\n\ta = 1\n\tb = 2\n)\n"
	d := makeDoc(t, src)
	ran := cursorAt(t, d, "a = 1", 0)
	a, ok := findAction(d.ExtractActions(common.UTF8, ran), KindExtractToNewFile)
	if !ok {
		t.Fatal("grouped var decl: no toNewFile action")
	}
	if !strings.HasSuffix(a.CreateFile.URI, "a.gox") {
		t.Fatalf("expected file named after first spec 'a', got %q", a.CreateFile.URI)
	}
}

func TestExtractComponentSkipsNamedReturn(t *testing.T) {
	src := "package demo\n\nimport \"github.com/doors-dev/gox\"\n\nfunc build() (acc gox.Elem) {\n\tacc = <b>x</b>\n\treturn <span>~(acc)</span>\n}\n"
	d := makeDoc(t, src)
	ran := cursorAt(t, d, "<span>~(acc)", 2)
	if _, ok := findAction(d.ExtractActions(common.UTF8, ran), KindExtractElem); ok {
		t.Fatal("must not offer extract-to-elem for markup capturing a named return")
	}
}

func TestExtractRoundTrips(t *testing.T) {
	cases := []struct {
		name     string
		marker   string
		into     int
		kind     string
		contains string
	}{
		{"var-template", "price * 2", 2, KindExtractVariable, "newVar"},
		{"var-snippet", `label := "$"`, 10, KindExtractVariable, "newVar"},
		{"const-snippet", `label := "$"`, 10, KindExtractConstant, "newConst"},
		{"var-toplevel", `Card("hello", 10)`, 2, KindExtractVariable, "newVar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := makeDoc(t, actionsSrc)
			ran := cursorAt(t, d, tc.marker, tc.into)
			actions := d.ExtractActions(common.UTF8, ran)
			a, ok := findAction(actions, tc.kind)
			if !ok {
				t.Fatalf("action %s not offered", tc.kind)
			}
			result := applyEdits(t, actionsSrc, a.Edits)
			target := reparse(t, result)
			if !strings.Contains(target, tc.contains) {
				t.Fatalf("generated target missing %q after extract.\n--- source ---\n%s\n--- target ---\n%s", tc.contains, result, target)
			}
		})
	}
}
