package workspace

import (
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

func TestSyntaxErrorsCleanSource(t *testing.T) {
	doc := makeDocLoose(t, sampleSrc)
	if errs := doc.SyntaxErrors(common.UTF8); len(errs) != 0 {
		t.Fatalf("clean source produced %d syntax errors: %+v", len(errs), errs)
	}
}

func TestSyntaxErrorsReportsErrorNode(t *testing.T) {
	const broken = `package demo

import "github.com/doors-dev/gox"

var x gox.Elem = <div class=>hi</div>
`
	errs := makeDocLoose(t, broken).SyntaxErrors(common.UTF8)
	if len(errs) == 0 {
		t.Fatal("broken source produced no syntax errors")
	}
	for _, e := range errs {
		if !e.Range.IsValid() {
			t.Fatalf("invalid range in %+v", e)
		}
		if e.Message == "" {
			t.Fatalf("empty message in %+v", e)
		}
	}
}

func TestSyntaxErrorsReportsMissingNode(t *testing.T) {
	const broken = `package demo

func f( {
}
`
	errs := makeDocLoose(t, broken).SyntaxErrors(common.UTF8)
	if len(errs) == 0 {
		t.Fatal("missing token produced no syntax errors")
	}
	found := false
	for _, e := range errs {
		if strings.HasPrefix(e.Message, "Syntax error: missing ") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a 'missing' diagnostic, got %+v", errs)
	}
}

func TestSyntaxErrorReportCleanSource(t *testing.T) {
	if r := makeDocLoose(t, sampleSrc).SyntaxErrorReport(); r != "" {
		t.Fatalf("clean source produced report: %q", r)
	}
	if r := SyntaxReport([]byte(sampleSrc)); r != "" {
		t.Fatalf("clean source produced standalone report: %q", r)
	}
}

func TestSyntaxErrorReportPositionAndNear(t *testing.T) {
	const broken = `package demo

import "github.com/doors-dev/gox"

var x gox.Elem = <div class=>hi</div>
`
	want := "5:28: syntax error near"
	report := makeDocLoose(t, broken).SyntaxErrorReport()
	if !strings.Contains(report, want) {
		t.Fatalf("Doc report = %q, want substring %q", report, want)
	}
	if standalone := SyntaxReport([]byte(broken)); standalone != report {
		t.Fatalf("standalone report = %q, want = %q", standalone, report)
	}
}

func TestSyntaxErrorReportMissingToken(t *testing.T) {
	const broken = `package demo

func f( {
}
`
	report := SyntaxReport([]byte(broken))
	if !strings.Contains(report, "syntax error: missing )") {
		t.Fatalf("report = %q, want a missing-token line", report)
	}
}

func syntaxErrorTexts(t *testing.T, src string) []string {
	t.Helper()
	lines := strings.Split(src, "\n")
	var out []string
	for _, e := range makeDocLoose(t, src).SyntaxErrors(common.UTF8) {
		if e.Range.Beg().Line() != e.Range.End().Line() {
			t.Fatalf("multi-line syntax error range %s", e.Range)
		}
		line := lines[e.Range.Beg().Line()]
		out = append(out, line[e.Range.Beg().Column():e.Range.End().Column()])
	}
	return out
}

func TestSyntaxErrorsFocusNestedErrorOnFirstLine(t *testing.T) {
	const broken = `package demo

import "github.com/doors-dev/gox"

elem Card(name string) {
	<div class=(name + >
		<h1>hi</h1>
	</div>
}

elem Other() {
	<p>ok</p>
}
`
	texts := syntaxErrorTexts(t, broken)
	if len(texts) == 0 || texts[0] != ">" {
		t.Fatalf("focus = %q, want first \">\"", texts)
	}
}

func TestSyntaxErrorsFocusClampsToFirstLine(t *testing.T) {
	const broken = `package demo

import "github.com/doors-dev/gox"

elem Card(name string) {
	<div class="card">
		~(if name != "" {
			<h1>hi</h1>
	</div>
}

elem Other() {
	<p>ok</p>
}
`
	texts := syntaxErrorTexts(t, broken)
	if len(texts) == 0 || texts[0] != `~(if name != "" {` {
		t.Fatalf("focus = %q, want the opener line", texts)
	}
}

func TestSyntaxErrorsFocusKeepsSingleLineError(t *testing.T) {
	const broken = `package demo

import "github.com/doors-dev/gox"

elem Card(name string) {
	<div class="card">
		<h1>~(name.</h1>
	</div>
}
`
	texts := syntaxErrorTexts(t, broken)
	if len(texts) == 0 || !strings.HasPrefix(texts[0], "~(name.") {
		t.Fatalf("focus = %q, want it to start at \"~(name.\"", texts)
	}
}

func TestStrayCloseTags(t *testing.T) {
	const src = `package demo

import "github.com/doors-dev/gox"

elem Card() {
	<div>hi</my-crad> <p>ok</p></ span ></div>
	<script></x></script>
}

var card = <section>é</x></section>
`
	doc := makeDoc(t, src)
	tags := doc.StrayCloseTags(common.UTF8)
	want := []struct {
		line, beg, end int
		message        string
	}{
		{5, 8, 18, "`</my-crad>` does not match an open element and is ignored."},
		{5, 28, 37, "`</span>` does not match an open element and is ignored."},
		{9, 22, 26, "`</x>` does not match an open element and is ignored."},
	}
	if len(tags) != len(want) {
		t.Fatalf("stray close tags = %+v, want %d", tags, len(want))
	}
	for i, w := range want {
		ran := common.NewRange(common.NewPos(w.line, w.beg), common.NewPos(w.line, w.end))
		if tags[i].Range != ran || tags[i].Message != w.message {
			t.Fatalf("tags[%d] = %+v, want %v %q", i, tags[i], ran, w.message)
		}
	}
	utf16 := doc.StrayCloseTags(common.UTF16)
	if got := utf16[2].Range; got != common.NewRange(common.NewPos(9, 21), common.NewPos(9, 25)) {
		t.Fatalf("UTF-16 range = %v, want 9:21-9:25", got)
	}
	if errs := doc.SyntaxErrors(common.UTF8); len(errs) != 0 {
		t.Fatalf("stray close tags produced syntax errors: %+v", errs)
	}
}

func TestStrayCloseTagsCleanAndNilDoc(t *testing.T) {
	if tags := makeDoc(t, sampleSrc).StrayCloseTags(common.UTF8); len(tags) != 0 {
		t.Fatalf("clean source produced stray close tags: %+v", tags)
	}
	var doc Doc
	if tags := doc.StrayCloseTags(common.UTF8); tags != nil {
		t.Fatalf("nil doc stray close tags = %+v, want nil", tags)
	}
}

func TestMissingPackageClauseIsSyntaxError(t *testing.T) {
	for name, src := range map[string]string{
		"empty":             "",
		"blank lines":       "\n\n",
		"comment only":      "// Package widgets.\n",
		"block comment":     "/* nothing */\n",
		"no package":        "import \"fmt\"\n\nfunc F() { fmt.Println() }\n",
		"elem only":         "elem Card() {\n\t<div></div>\n}\n",
		"package not first": "import \"fmt\"\n\npackage demo\n\nvar _ = fmt.Sprint\n",
	} {
		t.Run(name, func(t *testing.T) {
			doc := makeDocLoose(t, src)
			errs := doc.SyntaxErrors(common.UTF8)
			if len(errs) != 1 {
				t.Fatalf("syntax errors = %+v, want one", errs)
			}
			if errs[0].Range != common.NewRange(common.NewPos(0, 0), common.NewPos(0, 0)) || errs[0].Message != "Syntax error: missing package clause" {
				t.Fatalf("syntax error = %+v, want a missing package clause at 0:0", errs[0])
			}
			if report := SyntaxReport([]byte(src)); report != "  1:1: syntax error: missing package clause" {
				t.Fatalf("report = %q", report)
			}
			err := doc.Parse()
			if err == nil || !strings.Contains(err.Error(), "1:1: syntax error: missing package clause") {
				t.Fatalf("Parse() error = %v, want the missing package clause", err)
			}
		})
	}
}

func TestPackageClauseAfterCommentsIsAccepted(t *testing.T) {
	for _, src := range []string{
		"package demo\n",
		"// Package demo.\npackage demo\n",
		"//go:build linux\n\n/* doc */\npackage demo\n",
	} {
		doc := makeDocLoose(t, src)
		if errs := doc.SyntaxErrors(common.UTF8); len(errs) != 0 {
			t.Fatalf("SyntaxErrors(%q) = %+v, want none", src, errs)
		}
		if err := doc.Parse(); err != nil {
			t.Fatalf("Parse(%q) error = %v", src, err)
		}
	}
}

func TestMalformedPackageClauseReportsOnlyTheErrorNode(t *testing.T) {
	report := SyntaxReport([]byte("package\n\nelem Card() {\n\t<div></div>\n}\n"))
	if strings.Contains(report, "missing package clause") || !strings.Contains(report, "1:1: syntax error near") {
		t.Fatalf("report = %q, want only the error node", report)
	}
}

func TestMissingPackageClauseSortsBeforeOtherErrors(t *testing.T) {
	for _, src := range []string{
		"func F() int {\n\treturn = 1\n}\n",
		"elem Card(ps []gox.Proxy) {\n\t~>(ps...) <div>x</div>\n}\n",
	} {
		lines := strings.Split(SyntaxReport([]byte(src)), "\n")
		if len(lines) != 2 || lines[0] != "  1:1: syntax error: missing package clause" || !strings.HasPrefix(lines[1], "  2:") {
			t.Fatalf("report(%q) = %q, want the missing package clause, then the line 2 error", src, lines)
		}
	}
}
