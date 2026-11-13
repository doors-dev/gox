package processor

import (
	"errors"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const processorSample = `package demo

import "github.com/doors-dev/gox"

elem View(name string) {
	<div class="card">~(name)</div>
}
`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func asExit(t *testing.T, err error) *ExitError {
	t.Helper()
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("error = %v (%T), want *ExitError", err, err)
	}
	return ee
}

func TestNormalizePattern(t *testing.T) {
	supported := map[string]string{
		"...":              ".",
		"./...":            ".",
		".../":             ".",
		"foo/...":          "foo",
		"foo/.../":         "foo",
		"a/b/...":          "a/b",
		"./sub/...":        "./sub",
		"foo":              "foo",
		".":                ".",
		"./pkg":            "./pkg",
		"weird...name.gox": "weird...name.gox",
	}
	for in, want := range supported {
		got, ok := normalizePattern(in)
		if !ok || got != want {
			t.Errorf("normalizePattern(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}
	unsupported := []string{"a/.../b", "sub...", "net/http..."}
	for _, in := range unsupported {
		if _, ok := normalizePattern(in); ok {
			t.Errorf("normalizePattern(%q) ok = true, want false (unsupported pattern)", in)
		}
	}
}

func TestGenerateRejectsUnsupportedPatternClearly(t *testing.T) {
	err := Generate([]string{"a/.../b"}, false, false, false)
	if err == nil {
		t.Fatal("Generate(a/.../b) error = nil, want unsupported-pattern error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v, want a clear unsupported-pattern message", err)
	}
}

func TestFormatSupportsSingleGoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	writeFile(t, path, "package main\nfunc main(){println(\"x\")}\n")

	if err := Format([]string{path}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "package main\n\nfunc main() { println(\"x\") }\n"
	if got := string(content); got != want {
		t.Fatalf("formatted file = %q, want %q", got, want)
	}
}

func TestFormatRejectsUnsupportedSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path, "hello")

	err := Format([]string{path}, false, false, false, true)
	if err == nil {
		t.Fatal("Format() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "Expected a .go or .gox file") {
		t.Fatalf("Format() error = %v, want unsupported file error", err)
	}
}

func TestFormatNoGoLeavesGoFilesUntouched(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	unformatted := "package main\nfunc main(){println(\"x\")}\n"
	writeFile(t, goFile, unformatted)
	writeFile(t, filepath.Join(dir, "view.gox"), processorSample)

	if err := Format([]string{dir}, false, false, false, false); err != nil {
		t.Fatalf("Format(-no-go) error = %v", err)
	}

	content, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != unformatted {
		t.Fatalf("main.go = %q, want it untouched under -no-go", string(content))
	}
}

func TestFormatDefaultFormatsGoAndGox(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	writeFile(t, goFile, "package main\nfunc main(){println(\"x\")}\n")

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "package main\n\nfunc main() { println(\"x\") }\n"
	if string(content) != want {
		t.Fatalf("main.go = %q, want formatted %q", string(content), want)
	}
}

func TestFormatKeepsSpaceAfterImportName(t *testing.T) {
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "a.gox")
	writeFile(t, goxFile, "package main\n\nimport (\n\t_\"embed\"\n\tf\"fmt\"\n\t.\"strings\"\n)\n\nelem A() {\n\t<div>~(f.Sprint(Title(\"x\")))</div>\n}\n")

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"\t_ \"embed\"\n", "\tf \"fmt\"\n", "\t. \"strings\"\n"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("a.gox = %q, missing %q", string(content), want)
		}
	}
}

func TestFormatGenericMethods(t *testing.T) {
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "a.gox")
	writeFile(t, goxFile, "package main\n\nimport \"fmt\"\n\nfunc (s  Store[K])Get [T any]( key K,def T )T {\n\treturn def\n}\n\nelem (s *Store[K])Item [T fmt.Stringer](item T) {\n\t<li>~(item.String())</li>\n}\n\nfunc use(s Store[int]) string {\n\treturn s.Get[string](1,\"x\")\n}\n")

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func (s Store[K]) Get[T any](key K, def T) T {\n",
		"elem (s *Store[K]) Item[T fmt.Stringer](item T) {\n",
		"\treturn s.Get[string](1, \"x\")\n",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("a.gox = %q, missing %q", string(content), want)
		}
	}
	if err := Format([]string{dir}, false, false, true, true); err != nil {
		t.Fatalf("--check after format error = %v, want nil (idempotent)", err)
	}
}

func TestFormatSpacesTypeParameters(t *testing.T) {
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "a.gox")
	writeFile(t, goxFile, `package main

func F[T any,U comparable](t T, u U) {}

func G[ T ~int|~string,K,V any ](t T) {}

type S[K comparable,V any]struct {
	m Map[string,int]
}

type Map[K comparable,V any]map[K]V

type Alias[K comparable,V any] = Map[K,V]

func (s *S[K,V]) Map[R any,Q comparable](f func(V) R) []R {
	return nil
}

elem Fn[T any,U comparable](keys Map[K,T]) {
	<div attr=(Pick[K,T](keys))>~(Get[T,K](keys))</div>
}

elem (s *S[K,V]) Item[T any,U comparable](t T) {
	<li>~(t)</li>
}

func use() {
	f := Sum[int,float64]
	x := Map[string,int]{}
	_, _ = f, x
}
`)

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func F[T any, U comparable](t T, u U) {}\n",
		"func G[T ~int | ~string, K, V any](t T) {}\n",
		"type S[K comparable, V any] struct {\n\tm Map[string, int]\n}\n",
		"type Map[K comparable, V any] map[K]V\n",
		"type Alias[K comparable, V any] = Map[K, V]\n",
		"func (s *S[K, V]) Map[R any, Q comparable](f func(V) R) []R {\n",
		"elem Fn[T any, U comparable](keys Map[K, T]) {\n",
		"\t<div attr=(Pick[K, T](keys))>~(Get[T, K](keys))</div>\n",
		"elem (s *S[K, V]) Item[T any, U comparable](t T) {\n",
		"\tf := Sum[int, float64]\n",
		"\tx := Map[string, int]{}\n",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("a.gox = %q, missing %q", string(content), want)
		}
	}
	if err := Format([]string{dir}, false, false, true, true); err != nil {
		t.Fatalf("--check after format error = %v, want nil (idempotent)", err)
	}
}

func TestFormatJoinsMultiLineTypeLists(t *testing.T) {
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "a.gox")
	writeFile(t, goxFile, `package main

func Multi[
	T any, // first
	U comparable,
](t T, u U) {
}

type Long[
	// leading
	K comparable,
	V any,
] struct{}

type Ptr[P *C,] struct{}

var m Map[
	string,
	int,
]

var n Map[string,
	/* v */ int]

func use() {
	g := Sum[
		int,
		float64,
	]
	x := Make[
		string, // c
		int,
	](1)
	_, _ = g, x
}
`)

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	want := `package main

func Multi[T any, // first
U comparable,](t T, u U) {
}

type Long[
// leading
K comparable, V any,] struct{}

type Ptr[P *C,] struct{}

var m Map[string, int,]

var n Map[string,
/* v */ int]

func use() {
	g := Sum[int, float64,]
	x := Make[string, // c
	int,](1)
	_, _ = g, x
}
`
	if string(content) != want {
		t.Fatalf("a.gox = %q, want %q", string(content), want)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "a.go", content, parser.ParseComments); err != nil {
		t.Fatalf("formatted a.gox is not valid Go: %v", err)
	}
	if err := Format([]string{dir}, false, false, true, true); err != nil {
		t.Fatalf("--check after format error = %v, want nil (idempotent)", err)
	}
}

func TestFormatKeepsBodyIndentAfterBrokenTypeArguments(t *testing.T) {
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "a.gox")
	writeFile(t, goxFile, `package main

func Param(t Map[int,
	string]) {
	if true {
		return
	}
}

func Result() (Map[int,
	string], error) {
	if true {
		return nil, nil
	}
	return nil, nil
}

func (s *S[K,
	V]) Receiver() {
	if true {
		return
	}
}

func Constraint[T Iface[int,
	string]](t T) {
	if true {
		return
	}
}

elem Elem(t Map[int,
	string]) {
	~(if true {
		<div>~(t)</div>
	})
	<p>x</p>
}
`)

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	want := `package main

func Param(t Map[int, string]) {
	if true {
		return
	}
}

func Result() (Map[int, string], error) {
	if true {
		return nil, nil
	}
	return nil, nil
}

func (s *S[K, V]) Receiver() {
	if true {
		return
	}
}

func Constraint[T Iface[int, string]](t T) {
	if true {
		return
	}
}

elem Elem(t Map[int, string]) {
	~(if true {
		<div>~(t)</div>
	})
	<p>x</p>
}
`
	if string(content) != want {
		t.Fatalf("a.gox = %q, want %q", string(content), want)
	}
	if err := Format([]string{dir}, false, false, true, true); err != nil {
		t.Fatalf("--check after format error = %v, want nil (idempotent)", err)
	}
}

func TestFormatSpacesNameLists(t *testing.T) {
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "a.gox")
	writeFile(t, goxFile, `package main

type Pair struct {
	a,b int
}

var x,y = 1, 2

const c,d = 3, 4

func kind(v any) bool {
	switch v.(type) {
	case int,string:
		return true
	}
	return false
}
`)

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\ta, b int\n",
		"var x, y = 1, 2\n",
		"const c, d = 3, 4\n",
		"\tcase int, string:\n",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("a.gox = %q, missing %q", string(content), want)
		}
	}
}

func TestFormatAddsTrailingCommaToMultiLineLiterals(t *testing.T) {
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "a.gox")
	writeFile(t, goxFile, `package main

var a = []int{
	1,
	2
}

var b = map[string]int{
	"x": 1,
	"y": 2
}

var c = []int{
	1,
	2,
}

var d = []int{1, 2}

var e = [][]int{
	{1, 2},
	{
		3,
		4
	}
}

var f = []int{
	1,
	2 // last
}

elem view() {
	~(Props{
		A: 1,
		B: 2
	})
}
`)

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"var a = []int{\n\t1,\n\t2,\n}\n",
		"var b = map[string]int{\n\t\"x\": 1,\n\t\"y\": 2,\n}\n",
		"var c = []int{\n\t1,\n\t2,\n}\n",
		"var d = []int{1, 2}\n",
		"var e = [][]int{\n\t{1, 2},\n\t{\n\t\t3,\n\t\t4,\n\t},\n}\n",
		"var f = []int{\n\t1,\n\t2, // last\n}\n",
		"\t~Props{\n\t\tA: 1,\n\t\tB: 2,\n\t}\n",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("a.gox = %q, missing %q", string(content), want)
		}
	}
	if err := Format([]string{dir}, false, false, true, true); err != nil {
		t.Fatalf("--check after format error = %v, want nil (idempotent)", err)
	}
}

func TestFormatCheckReportsDriftAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	original := "package main\nfunc main(){println(\"x\")}\n"
	writeFile(t, goFile, original)

	err := Format([]string{dir}, false, false, true, true)
	if ee := asExit(t, err); ee.ExitCode() != 1 {
		t.Fatalf("--check exit code = %d, want 1", ee.ExitCode())
	}
	content, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Fatalf("main.go modified under --check: %q", string(content))
	}
}

func TestFormatCheckCleanExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() { println(\"x\") }\n")

	if err := Format([]string{dir}, false, false, true, true); err != nil {
		t.Fatalf("--check on clean tree error = %v, want nil", err)
	}
}

var noFinalNewlineSources = map[string]string{
	"named type":  "package p\n\ntype S int",
	"var":         "package p\n\nvar x int",
	"slice alias": "package p\n\ntype S = []int",
	"func type":   "package p\n\ntype F func() error",
	"pointer var": "package p\n\nvar p *int",
	"elem type":   "package p\n\ntype View = elem(title string)",
}

func TestFormatAddsFinalNewline(t *testing.T) {
	for name, src := range noFinalNewlineSources {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			goxFile := filepath.Join(dir, "a.gox")
			writeFile(t, goxFile, src)

			p := newProcessor(false, true, true, formatting)
			if ee := asExit(t, p.run([]string{dir}, false)); ee.ExitCode() != 1 || len(p.changed) != 1 {
				t.Fatalf("--check exit = %d, changed = %q, errors = %q, want exit 1 with one drift", ee.ExitCode(), p.changed, p.errors)
			}
			if err := Format([]string{dir}, false, false, false, true); err != nil {
				t.Fatalf("Format() error = %v", err)
			}
			content, err := os.ReadFile(goxFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != src+"\n" {
				t.Fatalf("a.gox = %q, want %q", string(content), src+"\n")
			}
		})
	}
}

var missingTokenSources = []struct {
	name   string
	src    string
	report string
}{
	{
		name:   "unclosed call",
		src:    "package demo\n\nfunc Make(n int) []int {\n\ts := make([]int, n\n}\n\nfunc Other() int {\n\tif true {\n\t\treturn 1\n\t}\n\treturn 2\n}\n",
		report: "4:20: syntax error: missing )",
	},
	{
		name:   "unclosed method call before elem",
		src:    "package demo\n\ntype ask struct {\n\tanswer answer\n}\n\nfunc (a *ask) submit(ctx context.Context, output string) {\n\ta.answer.Submit(ctx, output\n}\n\nelem (a *ask) form() {\n\t<form>\n\t\t<input name=\"q\"/>\n\t</form>\n}\n",
		report: "8:29: syntax error: missing )",
	},
	{
		name:   "unclosed composite literal",
		src:    "package demo\n\nfunc Get() []int {\n\treturn []int{1, 2, 3\n}\n",
		report: "6:1: syntax error: missing }",
	},
	{
		name:   "unclosed index",
		src:    "package demo\n\nfunc Get(s []int) int {\n\treturn s[0\n}\n\nelem Card(title string) {\n\t<div>~(title)</div>\n}\n",
		report: "4:12: syntax error: missing ]",
	},
	{
		name:   "unclosed attribute string",
		src:    "package demo\n\nelem Sort() {\n\t<th>\n\t\t<span class=\"sort-icon\" aria-hidden=true\">x</span>\n\t</th>\n\t<p>after</p>\n}\n",
		report: "5:43: syntax error: missing head_end",
	},
	{
		name:   "unclosed attribute string in self-closing tag",
		src:    "package demo\n\nelem Input() {\n\t<form>\n\t\t<input id=\"input type=\"text\" name=\"text\"/>\n\t</form>\n\t<p>after</p>\n}\n",
		report: "5:30: syntax error: missing self_closing_head_end",
	},
}

func TestFormatRefusesMissingTokens(t *testing.T) {
	for _, tc := range missingTokenSources {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			goxFile := filepath.Join(dir, "broken.gox")
			writeFile(t, goxFile, tc.src)

			err := Format([]string{dir}, false, false, false, true)
			if err == nil {
				t.Fatal("Format(broken) error = nil, want syntax error")
			}
			if !strings.Contains(err.Error(), "formatting error") || !strings.Contains(err.Error(), tc.report) {
				t.Fatalf("Format(broken) error = %v, want a formatting error with %q", err, tc.report)
			}
			content, err := os.ReadFile(goxFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != tc.src {
				t.Fatalf("broken.gox rewritten: %q", string(content))
			}
		})
	}
}

func TestFormatCheckMissingTokensExitsTwo(t *testing.T) {
	for _, tc := range missingTokenSources {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			goxFile := filepath.Join(dir, "broken.gox")
			writeFile(t, goxFile, tc.src)

			p := newProcessor(false, true, true, formatting)
			err := p.run([]string{dir}, false)
			if ee := asExit(t, err); ee.ExitCode() != 2 {
				t.Fatalf("--check exit = %d, want 2 (error, not drift)", ee.ExitCode())
			}
			if len(p.changed) != 0 {
				t.Fatalf("--check reported drift %q, want only the syntax error", p.changed)
			}
			if len(p.errors) != 1 || !strings.Contains(p.errors[0], tc.report) {
				t.Fatalf("--check errors = %q, want one with %q", p.errors, tc.report)
			}
			content, err := os.ReadFile(goxFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != tc.src {
				t.Fatalf("broken.gox modified under --check: %q", string(content))
			}
		})
	}
}

func TestFormatRefusesErrorNodes(t *testing.T) {
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "broken.gox")
	src := "package demo\n\nfunc Make(n int) []int {\n\ts := make([]int, n\n\treturn s\n}\n"
	writeFile(t, goxFile, src)

	err := Format([]string{dir}, false, false, false, true)
	if err == nil || !strings.Contains(err.Error(), "4:11: syntax error near") {
		t.Fatalf("Format(broken) error = %v, want a syntax error near 4:11", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != src {
		t.Fatalf("broken.gox rewritten: %q", string(content))
	}
}

func TestFormatDirectorySkipsIgnored(t *testing.T) {
	dir := t.TempDir()
	mainFile := filepath.Join(dir, "main.go")
	ignoredFile := filepath.Join(dir, "ignored.go")
	writeFile(t, mainFile, "package main\nfunc main(){println(\"x\")}\n")
	writeFile(t, ignoredFile, "package main\nfunc ignored(){println(\"x\")}\n")
	writeFile(t, filepath.Join(dir, ".gitignore"), "ignored.go\n")

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format(dir) error = %v", err)
	}

	mainContent, _ := os.ReadFile(mainFile)
	if got := string(mainContent); got != "package main\n\nfunc main() { println(\"x\") }\n" {
		t.Fatalf("main.go = %q", got)
	}
	ignoredContent, _ := os.ReadFile(ignoredFile)
	if got := string(ignoredContent); got != "package main\nfunc ignored(){println(\"x\")}\n" {
		t.Fatalf("ignored.go = %q, want untouched", got)
	}
}

func TestFormatRespectsRepoRootGitignoreFromSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".gitignore"), "ignored.go\n")
	sub := filepath.Join(root, "sub")
	mainFile := filepath.Join(sub, "main.go")
	ignoredFile := filepath.Join(sub, "ignored.go")
	unformatted := "package main\nfunc main(){println(\"x\")}\n"
	writeFile(t, mainFile, unformatted)
	writeFile(t, ignoredFile, unformatted)

	if err := Format([]string{sub}, false, false, false, true); err != nil {
		t.Fatalf("Format(sub) error = %v", err)
	}

	mainContent, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(mainContent); got != "package main\n\nfunc main() { println(\"x\") }\n" {
		t.Fatalf("sub/main.go = %q, want formatted", got)
	}
	ignoredContent, err := os.ReadFile(ignoredFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(ignoredContent) != unformatted {
		t.Fatalf("sub/ignored.go = %q, want untouched (repo-root .gitignore must apply)", string(ignoredContent))
	}
}

func TestGenerateRespectsRepoRootGitignoreFromSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".gitignore"), "ignored.gox\n")
	sub := filepath.Join(root, "sub")
	writeFile(t, filepath.Join(sub, "view.gox"), processorSample)
	writeFile(t, filepath.Join(sub, "ignored.gox"), processorSample)

	if err := Generate([]string{sub}, false, true, false); err != nil {
		t.Fatalf("Generate(sub) error = %v", err)
	}
	if !exists(filepath.Join(sub, "view.x.go")) {
		t.Fatal("view.x.go missing")
	}
	if exists(filepath.Join(sub, "ignored.x.go")) {
		t.Fatal("ignored.x.go generated despite repo-root .gitignore")
	}
}

func TestGenerateDirectoryCreatesTargetsAndRemovesOrphans(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "view.gox"), processorSample)
	writeFile(t, filepath.Join(dir, "orphan.x.go"), "// orphan")
	writeFile(t, filepath.Join(dir, ".gitignore"), "ignored.gox\n")
	writeFile(t, filepath.Join(dir, "ignored.gox"), processorSample)

	if err := Generate([]string{dir}, false, true, false); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !exists(filepath.Join(dir, "view.x.go")) {
		t.Fatal("generated target missing")
	}
	if exists(filepath.Join(dir, "orphan.x.go")) {
		t.Fatal("orphan target still exists")
	}
	if exists(filepath.Join(dir, "ignored.x.go")) {
		t.Fatal("ignored target unexpectedly exists")
	}
}

func TestGenerateSingleTargetRemovesOrphan(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ghost.x.go")
	writeFile(t, target, "// orphan")

	if err := Generate([]string{target}, false, false, false); err != nil {
		t.Fatalf("Generate(target) error = %v", err)
	}
	if exists(target) {
		t.Fatal("target still exists")
	}
}

func TestGenerateSingleSourceFileIgnoresGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".gitignore"), "*\n")
	view := filepath.Join(dir, "view.gox")
	writeFile(t, view, "package demo\n\nfunc View() {}\n")

	if err := Generate([]string{view}, false, false, false); err != nil {
		t.Fatalf("Generate(single file) error = %v", err)
	}
	if !exists(filepath.Join(dir, "view.x.go")) {
		t.Fatal("target missing for single-file generate")
	}
}

func TestGenerateAcceptsDotDotDotPattern(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "one.gox"), processorSample)
	writeFile(t, filepath.Join(root, "b", "two.gox"), processorSample)

	if err := Generate([]string{filepath.Join(root, "...")}, false, true, false); err != nil {
		t.Fatalf("Generate(dir/...) error = %v", err)
	}
	if !exists(filepath.Join(root, "a", "one.x.go")) || !exists(filepath.Join(root, "b", "two.x.go")) {
		t.Fatal("recursive ... pattern did not generate both targets")
	}
}

func TestGenerateAcceptsMultipleOperands(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "one.gox"), processorSample)
	writeFile(t, filepath.Join(root, "b", "two.gox"), processorSample)

	err := Generate([]string{filepath.Join(root, "a"), filepath.Join(root, "b")}, false, true, false)
	if err != nil {
		t.Fatalf("Generate(multi) error = %v", err)
	}
	if !exists(filepath.Join(root, "a", "one.x.go")) || !exists(filepath.Join(root, "b", "two.x.go")) {
		t.Fatal("multi-operand generate did not cover both dirs")
	}
}

func TestGenerateDedupesOverlappingOperands(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "one.gox"), processorSample)

	p := newProcessor(true, false, true, generation)
	if err := p.run([]string{root, root, filepath.Join(root, "...")}, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := p.updated.Load(); got != 1 {
		t.Fatalf("updated = %d, want 1 (overlapping operands must dedupe)", got)
	}
}

func TestCheckModeReportsFailureBanner(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "view.gox"), processorSample)

	p := newProcessor(false, true, true, generation)
	err := p.run([]string{dir}, false)
	if ee := asExit(t, err); ee.ExitCode() != 1 {
		t.Fatalf("exit = %d, want 1", ee.ExitCode())
	}
	if !p.failed() {
		t.Fatal("failed() = false, want true when --check finds drift")
	}
}

func TestGenerateCheckReportsStaleAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "view.gox"), processorSample)

	err := Generate([]string{dir}, false, false, true)
	if ee := asExit(t, err); ee.ExitCode() != 1 {
		t.Fatalf("--check exit = %d, want 1", ee.ExitCode())
	}
	if exists(filepath.Join(dir, "view.x.go")) {
		t.Fatal("--check wrote a target file")
	}
}

func TestGenerateCheckReportsOrphanWithoutRemoving(t *testing.T) {
	dir := t.TempDir()
	orphan := filepath.Join(dir, "ghost.x.go")
	writeFile(t, orphan, "// orphan")

	err := Generate([]string{dir}, false, false, true)
	if ee := asExit(t, err); ee.ExitCode() != 1 {
		t.Fatalf("--check exit = %d, want 1", ee.ExitCode())
	}
	if !exists(orphan) {
		t.Fatal("--check removed the orphan target")
	}
}

func TestGenerateCheckCleanExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "view.gox"), processorSample)
	if err := Generate([]string{dir}, false, true, false); err != nil {
		t.Fatalf("setup Generate() error = %v", err)
	}

	if err := Generate([]string{dir}, false, false, true); err != nil {
		t.Fatalf("--check on up-to-date tree error = %v, want nil", err)
	}
}

func TestGenerateCheckParseErrorExitsTwo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "broken.gox"), "package demo\n\nvar x int = = = 3\n")

	err := Generate([]string{dir}, false, false, true)
	if ee := asExit(t, err); ee.ExitCode() != 2 {
		t.Fatalf("--check parse error exit = %d, want 2", ee.ExitCode())
	}
}

func TestGenerateReportsParseErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "broken.gox"), "package demo\n\nvar x int = = = 3\n")

	err := Generate([]string{dir}, false, false, false)
	if err == nil {
		t.Fatal("Generate(broken) error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "parsing error") {
		t.Fatalf("Generate(broken) error = %v", err)
	}
}

func TestFormatPreservesModeAndLeavesNoTempFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not preserved meaningfully on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	writeFile(t, path, "package main\nfunc main(){println(\"x\")}\n")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Format([]string{path}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(content); got != "package main\n\nfunc main() { println(\"x\") }\n" {
		t.Fatalf("main.go = %q, want formatted", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".gox-fmt-") {
			t.Fatalf("stray temp file left behind: %s", e.Name())
		}
	}
}

func TestFormatSkipsHiddenDirectories(t *testing.T) {
	dir := t.TempDir()
	unformatted := "package main\nfunc main(){println(\"x\")}\n"
	hiddenFile := filepath.Join(dir, ".hidden", "x.go")
	writeFile(t, hiddenFile, unformatted)
	normalFile := filepath.Join(dir, "pkg", "y.go")
	writeFile(t, normalFile, unformatted)

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	hiddenContent, err := os.ReadFile(hiddenFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(hiddenContent) != unformatted {
		t.Fatalf(".hidden/x.go = %q, want untouched", string(hiddenContent))
	}
	normalContent, err := os.ReadFile(normalFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(normalContent); got != "package main\n\nfunc main() { println(\"x\") }\n" {
		t.Fatalf("pkg/y.go = %q, want formatted", got)
	}
}

func TestFormatCheckReportsUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() { println(\"x\") }\n")
	noperm := filepath.Join(dir, "noperm")
	if err := os.MkdirAll(noperm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(noperm, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(noperm, 0o755) })

	err := Format([]string{dir}, false, false, true, true)
	if ee := asExit(t, err); ee.ExitCode() != 2 {
		t.Fatalf("--check exit = %d, want 2 (unreadable dir must fail, not pass)", ee.ExitCode())
	}
}

func TestGenerateCheckReportsUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	noperm := filepath.Join(dir, "noperm")
	if err := os.MkdirAll(noperm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(noperm, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(noperm, 0o755) })

	err := Generate([]string{dir}, false, false, true)
	if ee := asExit(t, err); ee.ExitCode() != 2 {
		t.Fatalf("--check exit = %d, want 2 (unreadable dir must fail, not pass)", ee.ExitCode())
	}
}

func TestGenerateToleratesUnreadableDirDuringIgnoreScan(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.gox"), "package demo\n\nvar x = 1\n")
	noperm := filepath.Join(dir, ".sub", "noperm")
	if err := os.MkdirAll(noperm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(noperm, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(noperm, 0o755) })

	if err := Generate([]string{dir}, false, true, false); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !exists(filepath.Join(dir, "a.x.go")) {
		t.Fatal("target missing after tolerated ignore-scan error")
	}
}

var implicitCloseSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "sibling items",
		src:  "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>one\n\t\t<li>two\n\t</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t\tone\n\t\t</li><li>\n\t\t\ttwo\n\t\t</li>\n\t</ul>\n}\n",
	},
	{
		name: "nested closes at one position",
		src:  "package demo\n\nelem A() {\n\t<ul>\n\t\t<li><p>three\n\t</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t\t<p>\n\t\t\t\tthree\n\t\t\t</p>\n\t\t</li>\n\t</ul>\n}\n",
	},
	{
		name: "line comment before next item",
		src:  "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>one ~// c\n\t\t<li>two\n\t</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t\tone ~// c\n\t\t</li><li>\n\t\t\ttwo\n\t\t</li>\n\t</ul>\n}\n",
	},
	{
		name: "line comment on own line",
		src:  "package demo\n\nelem A() {\n\t<select>\n\t\t<option>a\n\t\t~// c\n\t\t<option>b\n\t</select>\n}\n",
		want: "package demo\n\nelem A() {\n\t<select>\n\t\t<option>\n\t\t\ta\n\t\t\t~// c\n\t\t</option><option>\n\t\t\tb\n\t\t</option>\n\t</select>\n}\n",
	},
	{
		name: "line comment before nested closes",
		src:  "package demo\n\nelem A() {\n\t<ul>\n\t\t<li><p>three ~// c\n\t</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t\t<p>\n\t\t\t\tthree ~// c\n\t\t\t</p>\n\t\t</li>\n\t</ul>\n}\n",
	},
	{
		name: "text keeps its trailing space",
		src:  "package demo\n\nelem A() {\n\t<table><tr><td>a <td>b </table>\n}\n",
		want: "package demo\n\nelem A() {\n\t<table><tr><td>a </td><td>b </td></tr></table>\n}\n",
	},
	{
		name: "inline list",
		src:  "package demo\n\nelem A() {\n\t<ul><li>Home <li>About</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul><li>Home </li><li>About</li></ul>\n}\n",
	},
	{
		name: "space filler before close",
		src:  "package demo\n\nelem A() {\n\t<table><tr><td>a  <td>b\t<td>c</table>\n}\n",
		want: "package demo\n\nelem A() {\n\t<table><tr><td>a </td><td>b</td><td>c</td></tr></table>\n}\n",
	},
	{
		name: "stray close before close",
		src:  "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>a </b>\n\t\t<li>b\n\t</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t\ta \n\t\t</li><li>\n\t\t\tb\n\t\t</li>\n\t</ul>\n}\n",
	},
	{
		name: "closing brace",
		src:  "package demo\n\nelem A(ok bool) {\n\t~(if ok {\n\t\t<p>open\n\t})\n\t~(if ok { <p>open })\n}\n",
		want: "package demo\n\nelem A(ok bool) {\n\t~(if ok {\n\t\t<p>\n\t\t\topen\n\t\t</p>\n\t})\n\t~(if ok {\n\t\t<p>open </p>\n\t})\n}\n",
	},
	{
		name: "text that looks like a comment",
		src:  "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>see http://x.y/z // c\n\t\t<li>b\n\t</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t\tsee http://x.y/z // c\n\t\t</li><li>\n\t\t\tb\n\t\t</li>\n\t</ul>\n}\n",
	},
	{
		name: "block and html comments",
		src:  "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>one ~/* c */\n\t\t<li>two <!-- d -->\n\t</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t\tone ~/* c */\n\t\t</li><li>\n\t\t\ttwo <!-- d -->\n\t\t</li>\n\t</ul>\n}\n",
	},
	{
		name: "empty items",
		src:  "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t<li>\n\t</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul>\n\t\t<li>\n\t\t</li><li>\n\t\t</li>\n\t</ul>\n}\n",
	},
	{
		name: "table rows",
		src:  "package demo\n\nelem A() {\n\t<table>\n\t\t<tr>\n\t\t\t<td>1\n\t\t\t<td>2\n\t\t<tr>\n\t\t\t<td>3\n\t</table>\n}\n",
		want: "package demo\n\nelem A() {\n\t<table>\n\t\t<tr>\n\t\t\t<td>\n\t\t\t\t1\n\t\t\t</td><td>\n\t\t\t\t2\n\t\t\t</td>\n\t\t</tr><tr>\n\t\t\t<td>\n\t\t\t\t3\n\t\t\t</td>\n\t\t</tr>\n\t</table>\n}\n",
	},
	{
		name: "table sections",
		src:  "package demo\n\nelem A() {\n\t<table>\n\t\t<thead>\n\t\t\t<tr>\n\t\t\t\t<th>a\n\t\t\t\t<th>b\n\t\t<tbody>\n\t\t\t<tr>\n\t\t\t\t<td>1\n\t\t\t\t<td>2\n\t\t\t<tr>\n\t\t\t\t<td>3\n\t</table>\n}\n",
		want: "package demo\n\nelem A() {\n\t<table>\n\t\t<thead>\n\t\t\t<tr>\n\t\t\t\t<th>\n\t\t\t\t\ta\n\t\t\t\t</th><th>\n\t\t\t\t\tb\n\t\t\t\t</th>\n\t\t\t</tr>\n\t\t</thead><tbody>\n\t\t\t<tr>\n\t\t\t\t<td>\n\t\t\t\t\t1\n\t\t\t\t</td><td>\n\t\t\t\t\t2\n\t\t\t\t</td>\n\t\t\t</tr><tr>\n\t\t\t\t<td>\n\t\t\t\t\t3\n\t\t\t\t</td>\n\t\t\t</tr>\n\t\t</tbody>\n\t</table>\n}\n",
	},
	{
		name: "paragraph closed by list item",
		src:  "package demo\n\nelem A() {\n\t<ul><li><p>a<li>b</ul>\n}\n",
		want: "package demo\n\nelem A() {\n\t<ul><li><p>a</p></li><li>b</li></ul>\n}\n",
	},
	{
		name: "options closed by hr",
		src:  "package demo\n\nelem A() {\n\t<select>\n\t\t<optgroup label=\"g\">\n\t\t\t<option>1\n\t\t\t<hr>\n\t\t\t<option>2\n\t</select>\n}\n",
		want: "package demo\n\nelem A() {\n\t<select>\n\t\t<optgroup label=\"g\">\n\t\t\t<option>\n\t\t\t\t1\n\t\t\t</option>\n\t\t</optgroup><hr>\n\t\t<option>\n\t\t\t2\n\t\t</option>\n\t</select>\n}\n",
	},
	{
		name: "one-line block before next item",
		src:  "package demo\n\nelem A(ok bool) {\n\t<ul>\n\t\t<li>~(if ok { <b>x</b> })\n\t\t<li>b\n\t</ul>\n}\n",
		want: "package demo\n\nelem A(ok bool) {\n\t<ul>\n\t\t<li>\n\t\t\t~(if ok {\n\t\t\t\t<b>x</b>\n\t\t\t})\n\t\t</li><li>\n\t\t\tb\n\t\t</li>\n\t</ul>\n}\n",
	},
}

func generatedMarkup(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "//line ") {
			lines = append(lines, line)
		}
	}
	src, err := format.Source([]byte(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

func checkFormat(t *testing.T, src, want string) {
	t.Helper()
	dir := t.TempDir()
	goxFile := filepath.Join(dir, "a.gox")
	target := filepath.Join(dir, "a.x.go")
	writeFile(t, goxFile, src)
	if err := Generate([]string{dir}, false, true, false); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	before := generatedMarkup(t, target)

	if err := Format([]string{dir}, false, false, false, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	content, err := os.ReadFile(goxFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("a.gox = %q, want %q", string(content), want)
	}
	if err := Format([]string{dir}, false, false, true, true); err != nil {
		t.Fatalf("--check after format error = %v, want nil (idempotent)", err)
	}
	if err := Generate([]string{dir}, false, true, false); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if after := generatedMarkup(t, target); after != before {
		t.Fatalf("generated code changed by formatting:\n%s\nwant:\n%s", after, before)
	}
}

func TestFormatInsertsImplicitCloses(t *testing.T) {
	for _, tc := range implicitCloseSources {
		t.Run(tc.name, func(t *testing.T) {
			checkFormat(t, tc.src, tc.want)
		})
	}
}

var implicitFragmentCloseSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "closed by ancestor close tag, brace and explicit",
		src:  "package demo\n\nelem A(ok bool) {\n\t<div><>a</div>\n\t~(if ok { <>x })\n\t<ul><><li>a<li>b</></ul>\n}\n",
		want: "package demo\n\nelem A(ok bool) {\n\t<div><>a</></div>\n\t~(if ok {\n\t\t<>x </>\n\t})\n\t<ul><><li>a</li><li>b</li></></ul>\n}\n",
	},
	{
		name: "closed by elem body brace",
		src:  "package demo\n\nelem B() {\n\t<p>z</p>\n\t<>tail\n}\n\nelem C() { <>x }\n",
		want: "package demo\n\nelem B() {\n\t<p>z</p>\n\t<>\n\t\ttail\n\t</>\n}\n\nelem C() {\n\t<>x </>\n}\n",
	},
	{
		name: "nested closes keep inner-first order",
		src:  "package demo\n\nelem A(ok bool) {\n\t<div><><b>a</div>\n\t<div><><>a</div>\n\t~(if ok { <><b><>x })\n}\n",
		want: "package demo\n\nelem A(ok bool) {\n\t<div><><b>a</b></></div>\n\t<div><><>a</></></div>\n\t~(if ok {\n\t\t<><b><>x </></b></>\n\t})\n}\n",
	},
	{
		name: "closed at end of file",
		src:  "package demo\n\nvar x = <>a",
		want: "package demo\n\nvar x = <>\n\ta\n</>\n",
	},
	{
		name: "content over several lines keeps the rest of the file in place",
		src:  "package demo\n\nelem A() {\n\t<div>\n\t\t<>\n\t\t\t<b>x</b>\n\t\t\t<i>y</i>\n\t</div>\n\t<p>after</p>\n}\n\nelem B() {\n\t<p>b</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>\n\t\t<>\n\t\t\t<b>x</b>\n\t\t\t<i>y</i>\n\t\t</>\n\t</div>\n\t<p>after</p>\n}\n\nelem B() {\n\t<p>b</p>\n}\n",
	},
}

func TestFormatClosesImplicitlyClosedFragments(t *testing.T) {
	for _, tc := range implicitFragmentCloseSources {
		t.Run(tc.name, func(t *testing.T) {
			checkFormat(t, tc.src, tc.want)
		})
	}
}

var trailingCommaSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "comment before comma",
		src:  "package demo\n\ntype T struct {\n\tX int\n\tTimeout int\n}\n\nvar a = T{\n\tX: 1,\n\tTimeout: 30 /* seconds */,\n}\n",
		want: "package demo\n\ntype T struct {\n\tX int\n\tTimeout int\n}\n\nvar a = T{\n\tX: 1,\n\tTimeout: 30 /* seconds */ ,\n}\n",
	},
	{
		name: "comment after comma",
		src:  "package demo\n\ntype T struct {\n\tX int\n}\n\nvar a = T{\n\tX: 1, /* c */\n}\n",
		want: "package demo\n\ntype T struct {\n\tX int\n}\n\nvar a = T{\n\tX: 1, /* c */\n}\n",
	},
	{
		name: "comments before comma",
		src:  "package demo\n\ntype T struct {\n\tX int\n}\n\nvar a = T{\n\tX: 1 /* a */ /* b */,\n}\n",
		want: "package demo\n\ntype T struct {\n\tX int\n}\n\nvar a = T{\n\tX: 1 /* a */ /* b */ ,\n}\n",
	},
	{
		name: "comments around comma",
		src:  "package demo\n\ntype T struct {\n\tX int\n}\n\nvar a = T{\n\tX: 1 /* c */, // d\n}\n",
		want: "package demo\n\ntype T struct {\n\tX int\n}\n\nvar a = T{\n\tX: 1 /* c */ , // d\n}\n",
	},
	{
		name: "map literal",
		src:  "package demo\n\nvar m = map[string]int{\n\t\"x\": 1,\n\t\"y\": 2 /* last */,\n}\n",
		want: "package demo\n\nvar m = map[string]int{\n\t\"x\": 1,\n\t\"y\": 2 /* last */ ,\n}\n",
	},
}

func TestFormatKeepsCommaAfterComments(t *testing.T) {
	for _, tc := range trailingCommaSources {
		t.Run(tc.name, func(t *testing.T) {
			checkFormat(t, tc.src, tc.want)
		})
	}
}

var blankLineSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "var group",
		src:  "package demo\n\nvar (\n\ta = 1\n\n\tb = 2\n\n\n\tc = 3\n\td = 4\n)\n",
		want: "package demo\n\nvar (\n\ta = 1\n\n\tb = 2\n\n\tc = 3\n\td = 4\n)\n",
	},
	{
		name: "const group",
		src:  "package demo\n\nconst (\n\tA = iota\n\n\t// B doc\n\tB\n\n\n\tC // c\n\tD\n)\n",
		want: "package demo\n\nconst (\n\tA = iota\n\n\t// B doc\n\tB\n\n\tC // c\n\tD\n)\n",
	},
	{
		name: "type group",
		src:  "package demo\n\ntype (\n\tX int\n\n\tY = string\n\n\n\tZ bool\n\tW byte\n)\n",
		want: "package demo\n\ntype (\n\tX int\n\n\tY = string\n\n\tZ bool\n\tW byte\n)\n",
	},
	{
		name: "struct fields",
		src:  "package demo\n\ntype S struct {\n\tA int\n\n\tB string\n\n\n\t// C doc\n\tC bool\n\tD byte\n}\n",
		want: "package demo\n\ntype S struct {\n\tA int\n\n\tB string\n\n\t// C doc\n\tC bool\n\tD byte\n}\n",
	},
	{
		name: "interface members",
		src:  "package demo\n\ntype I interface {\n\tM()\n\n\tN() int\n\n\n\tO()\n\tP()\n}\n",
		want: "package demo\n\ntype I interface {\n\tM()\n\n\tN() int\n\n\tO()\n\tP()\n}\n",
	},
	{
		name: "composite literals",
		src:  "package demo\n\nvar m = map[string][]int{\n\t\"a\": {1},\n\n\t\"b\": {\n\t\t2,\n\n\t\t3,\n\t}, // b\n\n\n\t\"c\": nil,\n\t\"d\": nil,\n\n\t\"e\": nil, // e\n}\n\nelem A() {\n\t~(map[string]int{\n\t\t\"x\": 1,\n\n\t\t\"y\": 2 /* y */,\n\t})\n}\n",
		want: "package demo\n\nvar m = map[string][]int{\n\t\"a\": {1},\n\n\t\"b\": {\n\t\t2,\n\n\t\t3,\n\t}, // b\n\n\t\"c\": nil,\n\t\"d\": nil,\n\n\t\"e\": nil, // e\n}\n\nelem A() {\n\t~map[string]int{\n\t\t\"x\": 1,\n\n\t\t\"y\": 2 /* y */ ,\n\t}\n}\n",
	},
}

func TestFormatKeepsBlankLinesBetweenEntries(t *testing.T) {
	for _, tc := range blankLineSources {
		t.Run(tc.name, func(t *testing.T) {
			checkFormat(t, tc.src, tc.want)
		})
	}
}

var tildeCommentSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "glued block comment",
		src:  "package demo\n\nelem A() {\n\t<p>two~/* c */three</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p>two~/* c */three</p>\n}\n",
	},
	{
		name: "spaced block comment",
		src:  "package demo\n\nelem A() {\n\t<p>two ~/* c */ three</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p>two ~/* c */ three</p>\n}\n",
	},
	{
		name: "line comment after text",
		src:  "package demo\n\nelem A() {\n\t<p>\n\t\tone ~// c\n\t\ttwo\n\t</p>\n\t<p>\n\t\tone~// c\n\t\ttwo\n\t</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p>\n\t\tone ~// c\n\t\ttwo\n\t</p>\n\t<p>\n\t\tone~// c\n\t\ttwo\n\t</p>\n}\n",
	},
	{
		name: "block comment ends line",
		src:  "package demo\n\nelem A() {\n\t<p>\n\t\tone ~/* c */\n\t\ttwo\n\t</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p>\n\t\tone ~/* c */\n\t\ttwo\n\t</p>\n}\n",
	},
	{
		name: "comments on own lines",
		src:  "package demo\n\nelem A() {\n\t<p>\n\t\tone\n\t\t~/* a */\n\n\t\t~/* b */\n\t\ttwo\n\t</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p>\n\t\tone\n\t\t~/* a */\n\n\t\t~/* b */\n\t\ttwo\n\t</p>\n}\n",
	},
	{
		name: "between elements",
		src:  "package demo\n\nelem A() {\n\t<p><b>x</b>~/* c */<i>y</i></p>\n\t<p>\n\t\t~(x) ~// c\n\t\t<b>y</b> ~// d\n\t</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p><b>x</b> ~/* c */ <i>y</i></p>\n\t<p>\n\t\t~(x) ~// c\n\t\t<b>y</b> ~// d\n\t</p>\n}\n",
	},
	{
		name: "multi-line block comment after expression",
		src:  "package demo\n\nelem A() {\n\t<div>\n\t\t~(x) ~/* note\n\t\tmore */\n\t</div>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>\n\t\t~(x) ~/* note\n\t\tmore */\n\t</div>\n}\n",
	},
	{
		name: "re-indented multi-line html comment",
		src:  "package demo\n\nelem A() {\n\t<div>\n<!-- c\nd -->\n\t</div>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>\n\t\t<!-- c\nd -->\n\t</div>\n}\n",
	},
	{
		name: "multi-line html comment in elem literal",
		src:  "package demo\n\nelem A() {\n\t~(elem() {\n\t\t<p>\n\t\t\t<!-- a\n\t\t\tb -->\n\t\t</p>\n\t})\n}\n",
		want: "package demo\n\nelem A() {\n\t~(elem() {\n\t\t<p>\n\t\t\t<!-- a\n\t\t\tb -->\n\t\t</p>\n\t})\n}\n",
	},
	{
		name: "blank line inside multi-line comments",
		src:  "package demo\n\nelem A() {\n\t<div>\n\t\t<!-- a\n\n\t\tb -->\n\t\t~/* c\n\n\t\td */\n\t</div>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>\n\t\t<!-- a\n\n\t\tb -->\n\t\t~/* c\n\n\t\td */\n\t</div>\n}\n",
	},
}

func TestFormatKeepsTildeCommentSpacing(t *testing.T) {
	for _, tc := range tildeCommentSources {
		t.Run(tc.name, func(t *testing.T) {
			checkFormat(t, tc.src, tc.want)
		})
	}
}

var redundantParenSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "comment before the value",
		src:  "package demo\n\nelem A() {\n\t<div>~(/* note */ \"Hi\")</div>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>~( /* note */ \"Hi\")</div>\n}\n",
	},
	{
		name: "comment after the value",
		src:  "package demo\n\nelem A() {\n\t<p>~(\"Hi\" /* c */)</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p>~(\"Hi\" /* c */ )</p>\n}\n",
	},
	{
		name: "comment in an attribute value",
		src:  "package demo\n\nelem A() {\n\t<p a=(/* c */ \"x\")></p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p a=( /* c */ \"x\")></p>\n}\n",
	},
	{
		name: "comment in a proxy",
		src:  "package demo\n\nelem A() {\n\t~>(/* c */ P{}) <b>p</b>\n}\n",
		want: "package demo\n\nelem A() {\n\t~>( /* c */ P{}) <b>p</b>\n}\n",
	},
	{
		name: "no comment",
		src:  "package demo\n\nelem A() {\n\t<div>~(\"Hi\")</div>\n\t<p a=(\"x\")></p>\n\t~>(P{}) <b>p</b>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>~\"Hi\"</div>\n\t<p a=\"x\"></p>\n\t~>P{} <b>p</b>\n}\n",
	},
}

func TestFormatRemovesRedundantParensWithoutComments(t *testing.T) {
	for _, tc := range redundantParenSources {
		t.Run(tc.name, func(t *testing.T) {
			checkFormat(t, tc.src, tc.want)
		})
	}
}

func TestFormatWritesLFForCRLFSource(t *testing.T) {
	checkFormat(t,
		"package demo\r\n\r\nelem A() {\r\n\t<div>\r\n\t\t<!-- a\r\n\t\tb -->\r\n\t\t~/* c\r\n\t\td */\r\n\t\t<:>e\r\n\t\tf</:>\r\n\t\t~(`g\r\nh`)\r\n\t</div>\r\n}\r\n",
		"package demo\n\nelem A() {\n\t<div>\n\t\t<!-- a\n\t\tb -->\n\t\t~/* c\n\t\td */\n\t\t<:>e\n\t\tf</:>\n\t\t~`g\nh`\n\t</div>\n}\n",
	)
}

func TestFormatKeepsCRBeforeCRLF(t *testing.T) {
	checkFormat(t,
		"package demo\r\n\r\nelem A() {\r\n\t<div>\r\n\t\t<!-- a\r\r\n\t\tb -->\r\n\t\t~/* c\r\r\n\t\td */\r\n\t\t<:>e\r\r\n\t\tf</:>\r\n\t\t~(`g\r\r\nh`)\r\n\t</div>\r\n}\r\n",
		"package demo\n\nelem A() {\n\t<div>\n\t\t<!-- a\r\r\n\t\tb -->\n\t\t~/* c\r\r\n\t\td */\n\t\t<:>e\r\r\n\t\tf</:>\n\t\t~`g\r\r\nh`\n\t</div>\n}\n",
	)
}

func TestFormatCRLFTildeLineComment(t *testing.T) {
	src := "package demo\n\nelem A(ok bool) {\n\t<div>\n\t\t~\n\t\t// c\n\t\t<p>y</p>\n\t</div>\n}\n"
	checkFormat(t, src, src)
	checkFormat(t, strings.ReplaceAll(src, "\n", "\r\n"), src)
}

func TestFormatCRLFStyleStringContinuation(t *testing.T) {
	src := "package demo\n\nelem A() {\n\t<style>\n\t\tp::before {\n\t\t\tcontent: \"a\\\n  b\";\n\t\t}\n\t</style>\n}\n"
	checkFormat(t, src, src)
	checkFormat(t, strings.ReplaceAll(src, "\n", "\r\n"), src)
}

func TestFormatCRLFScript(t *testing.T) {
	for _, src := range []string{
		"package demo\n\nelem A() {\n\t<script>\n\t\tlet s =\n\t\t\t\"a\\\n\t\tb\"\n\t\tgo(s)\n\t</script>\n}\n",
		"package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\n\t\tb\n  c`\n\t\tgo(s)\n\t</script>\n}\n",
		"package demo\n\nelem A() {\n\t<script>\n\t\tlet s = \"a\\\n\t\tb\"\n\t\tlet = = 1\n\t</script>\n}\n",
	} {
		checkFormat(t, src, src)
		checkFormat(t, strings.ReplaceAll(src, "\n", "\r\n"), src)
	}
}

var rawTextSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "trailing newline",
		src:  "package demo\n\nelem A() {\n\t<:>\na\n b\n</:>\n}\n",
		want: "package demo\n\nelem A() {\n\t<:>\na\n b\n</:>\n}\n",
	},
	{
		name: "only newlines",
		src:  "package demo\n\nelem A() {\n\t<div>\n\t\t<:>\n</:>\n\t\t<:>\n\n</:>\n\t</div>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>\n\t\t<:>\n</:>\n\t\t<:>\n\n</:>\n\t</div>\n}\n",
	},
	{
		name: "inline",
		src:  "package demo\n\nelem A() {\n\t<div>~(<:>a\n</:>)</div>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>\n\t\t~(<:>a\n</:>)\n\t</div>\n}\n",
	},
	{
		name: "crlf",
		src:  "package demo\r\n\r\nelem A() {\r\n\t<:>a\r\n b\r\n</:>\r\n}\r\n",
		want: "package demo\n\nelem A() {\n\t<:>a\n b\n</:>\n}\n",
	},
}

func TestFormatKeepsRawText(t *testing.T) {
	for _, tc := range rawTextSources {
		t.Run(tc.name, func(t *testing.T) {
			checkFormat(t, tc.src, tc.want)
		})
	}
}

func TestFormatKeepsTemplateLiteralContent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "column zero",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\nb\n  c`\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\nb\n  c`\n\t</script>\n}\n",
		},
		{
			name: "indented with spaces",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\n    b\n      c`\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\n    b\n      c`\n\t</script>\n}\n",
		},
		{
			name: "indented with tabs deeper than the script",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\n\t\t\t\tb\n\t\t\t\t\tc`\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\n\t\t\t\tb\n\t\t\t\t\tc`\n\t</script>\n}\n",
		},
		{
			name: "several templates",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet a = `x\n  y`\n\t\tlet b = 1\n\t\tlet c = `p\nq` + `r\n\ts`\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet a = `x\n  y`\n\t\tlet b = 1\n\t\tlet c =\n\t\t\t`p\nq` +\n\t\t\t`r\n\ts`\n\t</script>\n}\n",
		},
		{
			name: "substitutions spanning lines",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a ${\n  format(someLongArgumentName,\nanotherLongArgumentName, yetAnotherArgument)\n} d\n  e ${f} g\nh`\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a ${format(\n\t\t\tsomeLongArgumentName,\n\t\t\tanotherLongArgumentName,\n\t\t\tyetAnotherArgument,\n\t\t)} d\n  e ${f} g\nh`\n\t</script>\n}\n",
		},
		{
			name: "nested templates",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `<ul>\n  ${items.map((i) => `<li>\n\t${i}\n</li>`).join(\"\")}\n</ul>`\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `<ul>\n  ${items\n\t\t\t\t.map(\n\t\t\t\t\t(i) => `<li>\n\t${i}\n</li>`,\n\t\t\t\t)\n\t\t\t\t.join(\"\")}\n</ul>`\n\t</script>\n}\n",
		},
		{
			name: "tagged template",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tconst t = html`<div>\n  <b>${x}</b>\n</div>`\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tconst t = html`<div>\n  <b>${x}</b>\n</div>`\n\t</script>\n}\n",
		},
		{
			name: "template ends line and is followed by code",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet a = `\n  x\n`\n\t\tlet b = `y\n  z`.trim(); go(b)\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet a = `\n  x\n`\n\t\tlet b = `y\n  z`.trim()\n\t\tgo(b)\n\t</script>\n}\n",
		},
		{
			name: "deep in markup",
			in:   "package demo\n\nelem A() {\n\t<div>\n\t\t<section>\n\t\t\t<article>\n\t\t\t\t<script>\n\t\t\t\t\tlet s = `a\n  b\n\t\t\t\t\t\tc`\n\t\t\t\t</script>\n\t\t\t</article>\n\t\t</section>\n\t</div>\n}\n",
			want: "package demo\n\nelem A() {\n\t<div>\n\t\t<section>\n\t\t\t<article>\n\t\t\t\t<script>\n\t\t\t\t\tlet s = `a\n  b\n\t\t\t\t\t\tc`\n\t\t\t\t</script>\n\t\t\t</article>\n\t\t</section>\n\t</div>\n}\n",
		},
		{
			name: "invalid script",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\n  b`\n\t\tlet = = 1\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `a\n  b`\n\t\tlet = = 1\n\t</script>\n}\n",
		},
		{
			name: "style string continuation",
			in:   "package demo\n\nelem A() {\n\t<style>\n\t\t.a::before { content: \"a\\\n  b\"; }\n\t</style>\n}\n",
			want: "package demo\n\nelem A() {\n\t<style>\n\t\t.a::before {\n\t\t\tcontent: \"a\\\n  b\";\n\t\t}\n\t</style>\n}\n",
		},
		{
			name: "crlf style string continuation",
			in:   "package demo\r\n\r\nelem A() {\r\n\t<style>\r\n\t\tp::before { content: \"a\\\r\n  b\"; }\r\n\t</style>\r\n}\r\n",
			want: "package demo\n\nelem A() {\n\t<style>\n\t\tp::before {\n\t\t\tcontent: \"a\\\n  b\";\n\t\t}\n\t</style>\n}\n",
		},
		{
			name: "script string continuation",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet s =\n\t\t\t\"a\\\n  b\"\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s =\n\t\t\t\"a\\\n  b\"\n\t</script>\n}\n",
		},
		{
			name: "crlf script string continuation",
			in:   "package demo\r\n\r\nelem A() {\r\n\t<script>\r\n\t\tlet s = \"a\\\r\n\t\tb\"\r\n\t\tgo(s)\r\n\t</script>\r\n}\r\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s =\n\t\t\t\"a\\\n\t\tb\"\n\t\tgo(s)\n\t</script>\n}\n",
		},
		{
			name: "invalid script string continuation",
			in:   "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = \"a\\\n  b\"\n\t\tlet = = 1\n\t</script>\n}\n",
			want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = \"a\\\n  b\"\n\t\tlet = = 1\n\t</script>\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			goxFile := filepath.Join(dir, "a.gox")
			writeFile(t, goxFile, tc.in)

			if err := Format([]string{dir}, false, false, false, true); err != nil {
				t.Fatalf("Format() error = %v", err)
			}
			content, err := os.ReadFile(goxFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != tc.want {
				t.Fatalf("a.gox = %q, want %q", string(content), tc.want)
			}
			if err := Format([]string{dir}, false, false, true, true); err != nil {
				t.Fatalf("--check after format error = %v, want nil (idempotent)", err)
			}
		})
	}
}

var blockCommentSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "js column zero",
		src:  "package demo\n\nelem A() {\n\t<script>\n\t\t/* a\nb\n  c */\n\t\tlet x = 1\n\t</script>\n}\n",
		want: "package demo\n\nelem A() {\n\t<script>\n\t\t/* a\nb\n  c */\n\t\tlet x = 1\n\t</script>\n}\n",
	},
	{
		name: "js indented",
		src:  "package demo\n\nelem A() {\n\t<script>\n\t\t/* a\n\t\t   b\n\n\t\t   c */\n\t\tlet x = 1\n\t</script>\n}\n",
		want: "package demo\n\nelem A() {\n\t<script>\n\t\t/* a\n\t\t   b\n\n\t\t   c */\n\t\tlet x = 1\n\t</script>\n}\n",
	},
	{
		name: "js deeper than the script",
		src:  "package demo\n\nelem A() {\n\t<div>\n\t\t<script>\n\t\t\tfunction f() {\n\t\t\t\t/* a\n\t\t\t\t\t\t\t\tb */\n\t\t\t\treturn 1\n\t\t\t}\n\t\t</script>\n\t</div>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>\n\t\t<script>\n\t\t\tfunction f() {\n\t\t\t\t/* a\n\t\t\t\t\t\t\t\tb */\n\t\t\t\treturn 1\n\t\t\t}\n\t\t</script>\n\t</div>\n}\n",
	},
	{
		name: "js doc comments",
		src:  "package demo\n\nelem A() {\n\t<script>\n\t\t/**\n\t\t * Doc\n\t\t * @param x\n\t\t */\n\t\tfunction f(x) {}\n\t\t/*\n\t\t * star\n\t\t */\n\t\tlet x = 1\n\t</script>\n}\n",
		want: "package demo\n\nelem A() {\n\t<script>\n\t\t/**\n\t\t * Doc\n\t\t * @param x\n\t\t */\n\t\tfunction f(x) {}\n\t\t/*\n\t\t * star\n\t\t */\n\t\tlet x = 1\n\t</script>\n}\n",
	},
	{
		name: "js trailing and last comments",
		src:  "package demo\n\nelem A() {\n\t<script>\n\t\tlet x=1 /* a\n  b */\n\t\tfoo( )\n\t\t/* end\n  of script */\n\t</script>\n}\n",
		want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet x = 1 /* a\n  b */\n\t\tfoo()\n\t\t/* end\n  of script */\n\t</script>\n}\n",
	},
	{
		name: "js comment in template substitution",
		src:  "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `x ${/* c\n  d */ y} z`\n\t</script>\n}\n",
		want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet s = `x ${\n\t\t\t/* c\n  d */ y\n\t\t} z`\n\t</script>\n}\n",
	},
	{
		name: "invalid js",
		src:  "package demo\n\nelem A() {\n\t<script>\n\t\tlet = = 1\n\t\t/* a\nb */\n\t\tfoo()\n\t</script>\n}\n",
		want: "package demo\n\nelem A() {\n\t<script>\n\t\tlet = = 1\n\t\t/* a\nb */\n\t\tfoo()\n\t</script>\n}\n",
	},
	{
		name: "css column zero",
		src:  "package demo\n\nelem A() {\n\t<style>\n\t\t/* a\nb */\n\t\t.a {\n\t\t\tcolor: red;\n\t\t}\n\t</style>\n}\n",
		want: "package demo\n\nelem A() {\n\t<style>\n\t\t/* a\nb */\n\t\t.a {\n\t\t\tcolor: red;\n\t\t}\n\t</style>\n}\n",
	},
	{
		name: "css indented",
		src:  "package demo\n\nelem A() {\n\t<style>\n\t\t/*\n\t\t  a\n\t\t*/\n\t\t.a {\n\t\t\tcolor: red;\n\t\t}\n\t</style>\n}\n",
		want: "package demo\n\nelem A() {\n\t<style>\n\t\t/*\n\t\t  a\n\t\t*/\n\t\t.a {\n\t\t\tcolor: red;\n\t\t}\n\t</style>\n}\n",
	},
	{
		name: "css deeper than the style",
		src:  "package demo\n\nelem A() {\n\t<style>\n\t\t.a {\n\t\t\tcolor: red; /* x\n\t\t\t\t\t\t   y */\n\t\t}\n\t</style>\n}\n",
		want: "package demo\n\nelem A() {\n\t<style>\n\t\t.a {\n\t\t\tcolor: red; /* x\n\t\t\t\t\t\t   y */\n\t\t}\n\t</style>\n}\n",
	},
	{
		name: "css doc comments",
		src:  "package demo\n\nelem A() {\n\t<style>\n\t\t/**\n\t\t * Doc\n\t\t */\n\t\t.a {\n\t\t\tcolor: red;\n\t\t}\n\t\t/*\n\t\t * star\n\t\t */\n\t\t.b {\n\t\t\tcolor: blue;\n\t\t}\n\t</style>\n}\n",
		want: "package demo\n\nelem A() {\n\t<style>\n\t\t/**\n\t\t * Doc\n\t\t */\n\t\t.a {\n\t\t\tcolor: red;\n\t\t}\n\t\t/*\n\t\t * star\n\t\t */\n\t\t.b {\n\t\t\tcolor: blue;\n\t\t}\n\t</style>\n}\n",
	},
	{
		name: "invalid css",
		src:  "package demo\n\nelem A() {\n\t<style>\n\t\t.a { color: red;; }}\n\t\t/* a\n   b */\n\t</style>\n}\n",
		want: "package demo\n\nelem A() {\n\t<style>\n\t\t.a { color: red;; }}\n\t\t/* a\n   b */\n\t</style>\n}\n",
	},
	{
		name: "go in a tilde block",
		src:  "package demo\n\nelem A() {\n\t<p>\n\t\t~({\n\t\t\t/* a\n\t\t\t   b */\n\t\t\tx := 1\n\t\t\t_ = x\n\t\t})\n\t</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p>\n\t\t~({\n\t\t\t/* a\n\t\t\t   b */\n\t\t\tx := 1\n\t\t\t_ = x\n\t\t})\n\t</p>\n}\n",
	},
	{
		name: "go in a closure argument",
		src:  "package demo\n\nfunc F() {\n\tg(func() {\n\t\t/* a\n\n\t\t   b */\n\t})\n}\n",
		want: "package demo\n\nfunc F() {\n\tg(func() {\n\t\t/* a\n\n\t\t   b */\n\t})\n}\n",
	},
	{
		name: "go before a tilde value",
		src:  "package demo\n\nelem A() {\n\t<p>~(/* a\n b */ \"Hi\")</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<p>\n\t\t~( /* a\n b */ \"Hi\")\n\t</p>\n}\n",
	},
}

func TestFormatKeepsBlockCommentContent(t *testing.T) {
	for _, tc := range blockCommentSources {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			goxFile := filepath.Join(dir, "a.gox")
			writeFile(t, goxFile, tc.src)

			if err := Format([]string{dir}, false, false, false, true); err != nil {
				t.Fatalf("Format() error = %v", err)
			}
			content, err := os.ReadFile(goxFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != tc.want {
				t.Fatalf("a.gox = %q, want %q", string(content), tc.want)
			}
			if err := Format([]string{dir}, false, false, true, true); err != nil {
				t.Fatalf("--check after format error = %v, want nil (idempotent)", err)
			}
		})
	}
}

var emptyLineSources = []struct {
	name string
	src  string
	want string
}{
	{
		name: "import group",
		src:  "package demo\n\nimport (\n\t\"fmt\"\n\t\n\t\"strings\"\n\n\t\"os\"\n)\n\nvar _ = fmt.Sprint(strings.ToUpper(\"\"), os.Args)\n",
		want: "package demo\n\nimport (\n\t\"fmt\"\n\n\t\"strings\"\n\n\t\"os\"\n)\n\nvar _ = fmt.Sprint(strings.ToUpper(\"\"), os.Args)\n",
	},
	{
		name: "func body",
		src:  "package demo\n\nfunc F() int {\n\ta := 1\n\t\n\tif a > 0 {\n\t\tb := 2\n\t\t\n\t\ta += b\n\n\t\ta++\n\t}\n\n\treturn a\n}\n",
		want: "package demo\n\nfunc F() int {\n\ta := 1\n\n\tif a > 0 {\n\t\tb := 2\n\n\t\ta += b\n\n\t\ta++\n\t}\n\n\treturn a\n}\n",
	},
	{
		name: "element children",
		src:  "package demo\n\nelem A() {\n\t<div>\n\t\t<p>a</p>\n\t\t\n\t\t<ul>\n\t\t\t<li>x</li>\n\t\t\t\n\t\t\t<li>y</li>\n\n\t\t\t<li>z</li>\n\t\t</ul>\n\t</div>\n\n\t<p>b</p>\n}\n",
		want: "package demo\n\nelem A() {\n\t<div>\n\t\t<p>a</p>\n\n\t\t<ul>\n\t\t\t<li>x</li>\n\n\t\t\t<li>y</li>\n\n\t\t\t<li>z</li>\n\t\t</ul>\n\t</div>\n\n\t<p>b</p>\n}\n",
	},
}

func TestFormatWritesEmptyBlankLines(t *testing.T) {
	for _, tc := range emptyLineSources {
		t.Run(tc.name, func(t *testing.T) {
			checkFormat(t, tc.src, tc.want)
		})
	}
}

func TestGenerateRefusesMissingPackageClauseAndKeepsTarget(t *testing.T) {
	for name, src := range map[string]string{
		"empty":        "",
		"comment only": "// Package widgets.\n",
		"no package":   "elem Table() {\n\t<table></table>\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "table.gox")
			target := filepath.Join(dir, "table.x.go")
			writeFile(t, source, src)
			writeFile(t, target, "// old target\n")

			err := Generate([]string{dir}, false, false, false)
			if err == nil || !strings.Contains(err.Error(), "parsing error") || !strings.Contains(err.Error(), "1:1: syntax error: missing package clause") {
				t.Fatalf("Generate() error = %v, want a missing package clause parsing error", err)
			}
			content, err := os.ReadFile(target)
			if err != nil || string(content) != "// old target\n" {
				t.Fatalf("table.x.go = %q, %v, want the old target kept", content, err)
			}
			if ee := asExit(t, Generate([]string{dir}, false, false, true)); ee.ExitCode() != 2 {
				t.Fatalf("--check exit = %d, want 2", ee.ExitCode())
			}
			if err := Format([]string{dir}, false, false, false, true); err != nil {
				t.Fatalf("Format() error = %v, want formatting unchanged", err)
			}
		})
	}
}
