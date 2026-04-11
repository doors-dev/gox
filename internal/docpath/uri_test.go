package docpath

import (
	"strings"
	"testing"
)

func TestParseDocumentURIEmpty(t *testing.T) {
	got, err := ParseDocumentURI("")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestParseDocumentURIRejectsNonFile(t *testing.T) {
	_, err := ParseDocumentURI("https://example.com/x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseDocumentURIAddsSlash(t *testing.T) {
	// Two-slash form should be canonicalized to three slashes.
	got, err := ParseDocumentURI("file://Users/x/y.go")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(string(got), "file:///") {
		t.Fatalf("got %q", got)
	}
}

func TestParseDocumentURIWindowsDriveUppercase(t *testing.T) {
	got, err := ParseDocumentURI("file:///c:/x/y.go")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(string(got), "/C:/") {
		t.Fatalf("got %q, want uppercase drive", got)
	}
}

func TestURIFromPathEmpty(t *testing.T) {
	if got := URIFromPath(""); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestURIFromPathPosix(t *testing.T) {
	got := URIFromPath("/tmp/foo.go")
	if got != "file:///tmp/foo.go" {
		t.Fatalf("got %q", got)
	}
}

func TestURIFromPathWindowsDrive(t *testing.T) {
	got := URIFromPath("c:/x/y.go")
	if !strings.HasPrefix(string(got), "file:///C:/") {
		t.Fatalf("got %q", got)
	}
}

func TestDocumentURIPath(t *testing.T) {
	uri := URIFromPath("/tmp/foo.go")
	if got := uri.Path(); got != "/tmp/foo.go" {
		t.Fatalf("got %q", got)
	}
}

func TestDocumentURIBaseAndDir(t *testing.T) {
	uri := URIFromPath("/tmp/sub/file.gox")
	if uri.Base() != "file.gox" {
		t.Fatalf("Base = %q", uri.Base())
	}
	if uri.DirPath() != "/tmp/sub" {
		t.Fatalf("DirPath = %q", uri.DirPath())
	}
	if got := uri.Dir(); !strings.HasSuffix(string(got), "/tmp/sub") {
		t.Fatalf("Dir = %q", got)
	}
}

func TestDocumentURIClean(t *testing.T) {
	uri := URIFromPath("/tmp/sub/../file.gox")
	cleaned := uri.Clean()
	if !strings.HasSuffix(string(cleaned), "/tmp/file.gox") {
		t.Fatalf("Clean = %q", cleaned)
	}
}

func TestDocumentURIUnmarshalText(t *testing.T) {
	var uri DocumentURI
	if err := uri.UnmarshalText([]byte("file:///tmp/x.go")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(uri), "/tmp/x.go") {
		t.Fatalf("got %q", uri)
	}
}

func TestDocumentURIPathEmpty(t *testing.T) {
	var uri DocumentURI
	if got := uri.Path(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestIsWindowsDrivePath(t *testing.T) {
	cases := map[string]bool{
		"":          false,
		"a":         false,
		"ab":        false,
		"c:":        false,
		"c:/":       true,
		"C:/foo":    true,
		"3:/foo":    false, // first char must be letter
		"/c:/foo":   false,
		"/tmp/foo":  false,
	}
	for in, want := range cases {
		if got := isWindowsDrivePath(in); got != want {
			t.Errorf("isWindowsDrivePath(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsWindowsDriveURIPath(t *testing.T) {
	cases := map[string]bool{
		"":            false,
		"/c:":         false,
		"/c:/":        true,
		"/C:/foo":    true,
		"//c:/foo":    false,
		"/3:/foo":     false,
	}
	for in, want := range cases {
		if got := isWindowsDriveURIPath(in); got != want {
			t.Errorf("isWindowsDriveURIPath(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseDocumentURISlowPathPercent(t *testing.T) {
	// Force the slow path with a percent escape and verify it round-trips.
	got, err := ParseDocumentURI("file:///tmp/has%20space.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "has%20space.go") &&
		!strings.Contains(string(got), "has space.go") {
		t.Fatalf("got %q", got)
	}
}
