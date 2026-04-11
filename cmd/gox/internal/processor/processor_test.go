package processor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const processorSample = `package demo

import "github.com/doors-dev/gox"

elem View(name string) {
	<div class="card">~(name)</div>
}
`

func TestFormatSupportsSingleGoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main(){println(\"x\")}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Format(path, false, false); err != nil {
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
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Format(path, false, false)
	if err == nil {
		t.Fatal("Format() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "Expected a .go or .gox file") {
		t.Fatalf("Format() error = %v, want unsupported file error", err)
	}
}

func TestGenerateSupportsSingleSourceFileWithoutIgnoreSetup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "view.gox")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package demo\n\nfunc View() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := newProcessor(path, false, false, generation); err != nil {
		t.Fatalf("newProcessor() error = %v", err)
	}
}

func TestGenerateDirectoryCreatesTargetsAndRemovesOrphans(t *testing.T) {
	dir := t.TempDir()
	view := filepath.Join(dir, "view.gox")
	if err := os.WriteFile(view, []byte(processorSample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orphan.x.go"), []byte("// orphan"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.gox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.gox"), []byte(processorSample), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(dir, false, true); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "view.x.go")); err != nil {
		t.Fatalf("generated target missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "orphan.x.go")); !os.IsNotExist(err) {
		t.Fatalf("orphan target still exists, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ignored.x.go")); !os.IsNotExist(err) {
		t.Fatalf("ignored target unexpectedly exists, err = %v", err)
	}
}

func TestGenerateSingleTargetRemovesOrphan(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ghost.x.go")
	if err := os.WriteFile(target, []byte("// orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(target, false, false); err != nil {
		t.Fatalf("Generate(target) error = %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists, err = %v", err)
	}
}

func TestFormatDirectoryFormatsGoFilesAndSkipsIgnored(t *testing.T) {
	dir := t.TempDir()
	mainFile := filepath.Join(dir, "main.go")
	ignoredFile := filepath.Join(dir, "ignored.go")
	if err := os.WriteFile(mainFile, []byte("package main\nfunc main(){println(\"x\")}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignoredFile, []byte("package main\nfunc ignored(){println(\"x\")}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Format(dir, false, false); err != nil {
		t.Fatalf("Format(dir) error = %v", err)
	}

	mainContent, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	wantMain := "package main\n\nfunc main() { println(\"x\") }\n"
	if got := string(mainContent); got != wantMain {
		t.Fatalf("main.go = %q, want %q", got, wantMain)
	}

	ignoredContent, err := os.ReadFile(ignoredFile)
	if err != nil {
		t.Fatal(err)
	}
	wantIgnored := "package main\nfunc ignored(){println(\"x\")}\n"
	if got := string(ignoredContent); got != wantIgnored {
		t.Fatalf("ignored.go = %q, want %q", got, wantIgnored)
	}
}

func TestGenerateReportsParseErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.gox")
	if err := os.WriteFile(path, []byte("package demo\n\nelem Broken() {\n\t<div>\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Generate(path, false, false)
	if err == nil {
		t.Fatal("Generate(broken) error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "parsing error") {
		t.Fatalf("Generate(broken) error = %v", err)
	}
}
