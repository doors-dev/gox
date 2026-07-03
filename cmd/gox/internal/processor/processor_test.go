package processor

import (
	"errors"
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
