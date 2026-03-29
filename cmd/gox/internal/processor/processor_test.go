package processor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if !strings.Contains(string(content), "func main() {") {
		t.Fatalf("formatted file did not change as expected:\n%s", content)
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
