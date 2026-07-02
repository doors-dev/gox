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
