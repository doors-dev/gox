package workspace

import (
	"os"
	"strings"
	"testing"
)

func reloadTree(t *testing.T) (child, source, target string) {
	t.Helper()
	_, child, source = nestedRootsTree(t)
	return child, source, strings.TrimSuffix(source, ".gox") + ".x.go"
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A document without an open buffer follows its file when reloaded: the
// copy, the generated code and the .x.go on disk change, and the manager
// reports it once.
func TestManagerReloadFollowsFile(t *testing.T) {
	child, source, target := reloadTree(t)
	m := lockedManager(t)
	reloads := 0
	m.OnReload(func(Doc) {
		reloads++
	})
	m.AddWorkspace(rootURI(child))
	doc, _ := m.Doc(rootURI(source))
	if doc == nil || doc.Err() != nil {
		t.Fatalf("Doc(%s) = %v", source, doc)
	}
	if m.Reload(doc) || reloads != 0 {
		t.Fatal("reload reported a change for an unchanged file")
	}

	changed := strings.Replace(sampleSrc, "hello", "ondisk", 1)
	writeTestFile(t, source, changed)
	if !m.Reload(doc) || reloads != 1 {
		t.Fatalf("reload of a changed file: reloads = %d, want 1", reloads)
	}
	if !strings.Contains(doc.TargetContent(), "ondisk") {
		t.Fatal("reload kept the old text")
	}
	if got := readTestFile(t, target); got != doc.TargetContent() {
		t.Fatalf("reload did not write the .x.go:\n%s", got)
	}
	if m.Reload(doc) {
		t.Fatal("second reload reported a change")
	}
	writeTestFile(t, source, strings.ReplaceAll(changed, "\n", "\r\n"))
	if m.Reload(doc) {
		t.Fatal("reload reported a change for the same text with CRLF line endings")
	}

	doc.SourceOpen(1)
	writeTestFile(t, source, sampleSrc)
	if m.Reload(doc) || !strings.Contains(doc.TargetContent(), "ondisk") {
		t.Fatal("reload replaced an open buffer")
	}
	doc.SourceClose()
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if m.Reload(doc) || reloads != 1 {
		t.Fatalf("reload of a missing file: reloads = %d, want 1", reloads)
	}
}

// Closing a buffer without saving returns the .x.go on disk to the file too,
// e.g. after the ticker restored it from the buffer.
func TestDocSourceCloseRewritesTarget(t *testing.T) {
	child, source, target := reloadTree(t)
	m := lockedManager(t)
	m.AddWorkspace(rootURI(child))
	doc := openSource(t, m, source, strings.Replace(sampleSrc, "hello", "discarded", 1))
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	if !doc.SourceClose() {
		t.Fatal("SourceClose reported no change for a discarded buffer")
	}
	if got := readTestFile(t, target); strings.Contains(got, "discarded") || !strings.Contains(got, "hello") {
		t.Fatalf("the .x.go keeps the discarded text:\n%s", got)
	}
}

// The ticker restores a missing .x.go from the file, not from a buffer that
// was closed while the file was missing.
func TestTickerRestoresTargetFromFileAfterDiscardedClose(t *testing.T) {
	child, source, target := reloadTree(t)
	m := lockedManager(t)
	reloads := 0
	m.OnReload(func(Doc) {
		reloads++
	})
	m.AddWorkspace(rootURI(child))
	doc := openSource(t, m, source, strings.Replace(sampleSrc, "hello", "discarded", 1))
	tick := func() {
		m.dirs[child].ProcessFileRemovals(m.ghost, m.Reload)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	tick()
	tick()
	if _, err := os.Stat(target); err == nil {
		t.Fatal("the .x.go of a missing source was kept")
	}
	if doc.SourceClose() {
		t.Fatal("closing with the file missing reverted the document")
	}
	writeTestFile(t, source, sampleSrc)
	tick()
	if got := readTestFile(t, target); strings.Contains(got, "discarded") || !strings.Contains(got, "hello") {
		t.Fatalf("the restored .x.go has the discarded text:\n%s", got)
	}
	if reloads != 1 {
		t.Fatalf("reloads = %d, want 1", reloads)
	}
}
