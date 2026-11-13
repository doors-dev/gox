package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func lockedManager(t *testing.T) Manager {
	t.Helper()
	m := NewManager()
	m.Lock()
	t.Cleanup(func() {
		m.Unlock()
		m.StopAll()
	})
	return m
}

func openSource(t *testing.T, m Manager, path, buffer string) Doc {
	t.Helper()
	doc, _ := m.Doc(rootURI(path))
	if doc == nil || doc.Err() != nil {
		t.Fatalf("Doc(%s) = %v", path, doc)
	}
	if _, err := doc.SourceUpdate(buffer); err != nil {
		t.Fatal(err)
	}
	doc.SourceOpen(1)
	return doc
}

func released(d Doc) bool { return d.parser == nil }

// Folder changes never replace or release a document with an open buffer;
// documents without one move to the root that covers them, or are released.
func TestManagerFolderChangesKeepOpenDocuments(t *testing.T) {
	buffer := strings.Replace(sampleSrc, "hello", "frombuffer", 1)
	parent, child, source := nestedRootsTree(t)
	idlePath := filepath.Join(child, "idle.gox")
	outsidePath := filepath.Join(parent, "outside.gox")
	for _, p := range []string{idlePath, outsidePath} {
		if err := os.WriteFile(p, []byte(sampleSrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := lockedManager(t)
	same := func(step string, want Doc) {
		t.Helper()
		if got, _ := m.Doc(rootURI(source)); got != want || !got.SourceIsOpened() || !strings.Contains(got.TargetContent(), "frombuffer") || released(got) {
			t.Fatalf("%s: open document replaced or changed: %v", step, got)
		}
	}

	m.AddWorkspace(rootURI(child)) // initialize, child first
	open := openSource(t, m, source, buffer)
	idle, _ := m.Doc(rootURI(idlePath))
	m.AddWorkspace(rootURI(parent))
	same("parent added", open)
	if got, _ := m.Doc(rootURI(idlePath)); got != idle {
		t.Fatal("parent reloaded an idle document of the child instead of taking it over")
	}

	outside, _ := m.Doc(rootURI(outsidePath))
	m.AddWorkspace(rootURI(child))
	m.RemoveWorkspace(rootURI(parent)) // one event: parent replaced by child
	if roots := m.roots(); len(roots) != 1 || roots[0] != child {
		t.Fatalf("roots = %v, want [%s]", roots, child)
	}
	same("parent replaced by child", open)
	if !released(outside) {
		t.Fatal("idle document no root covers any more was not released")
	}

	m.RemoveWorkspace(rootURI(child))
	same("folder removed", open)
	if got, _ := m.Doc(rootURI(idlePath)); got != nil || !released(idle) {
		t.Fatal("idle document of a removed folder is still served or not released")
	}
	m.AddWorkspace(rootURI(child))
	same("folder re-added", open)

	m.RemoveWorkspace(rootURI(child))
	open.SourceClose()
	m.release()
	if got, _ := m.Doc(rootURI(source)); got != nil || !released(open) {
		t.Fatal("document of a removed folder was not released after its buffer closed")
	}
}

// Closing the source buffer returns the document to the file.
func TestDocSourceCloseReturnsToFile(t *testing.T) {
	parent, _, source := nestedRootsTree(t)
	m := lockedManager(t)
	m.AddWorkspace(rootURI(parent))
	doc := openSource(t, m, source, strings.Replace(sampleSrc, "hello", "discarded", 1))
	if !doc.SourceClose() {
		t.Fatal("SourceClose reported no change for a discarded buffer")
	}
	if strings.Contains(doc.TargetContent(), "discarded") {
		t.Fatal("document keeps the discarded text")
	}
	if doc.SourceClose() {
		t.Fatal("SourceClose reported a change for a buffer equal to the file")
	}
	crlf := strings.ReplaceAll(sampleSrc, "\n", "\r\n")
	if err := os.WriteFile(source, []byte(crlf), 0o644); err != nil {
		t.Fatal(err)
	}
	openSource(t, m, source, crlf)
	if doc.SourceClose() {
		t.Fatal("SourceClose reported a change for a buffer equal to a CRLF file")
	}
}

// A re-added folder discovers the documents around a document kept open.
func TestManagerReaddedFolderRediscoversAndTickerReleases(t *testing.T) {
	parent, _, source := nestedRootsTree(t)
	top := filepath.Join(parent, "top.gox")
	sib := filepath.Join(parent, "sib.gox")
	for _, p := range []string{top, sib} {
		if err := os.WriteFile(p, []byte(sampleSrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := lockedManager(t)
	m.AddWorkspace(rootURI(parent))
	open, _ := m.Doc(rootURI(top))
	open.SourceOpen(1)
	m.RemoveWorkspace(rootURI(parent))
	if _, ok := m.dirs[filepath.Dir(source)]; ok {
		t.Fatal("idle dir of a removed folder kept")
	}
	m.AddWorkspace(rootURI(parent))
	d, ok := m.dirs[filepath.Dir(source)]
	if !ok || d.docs[strings.TrimSuffix(source, ".gox")] == nil {
		t.Fatal("re-added folder did not rediscover the documents under the kept dir")
	}
	if d, ok := m.dirs[parent]; !ok || d.docs[strings.TrimSuffix(sib, ".gox")] == nil {
		t.Fatal("re-added folder did not rediscover the documents next to the open one")
	}
	m.RemoveWorkspace(rootURI(parent))
	open.SourceClose()
	m.Unlock()
	time.Sleep(600 * time.Millisecond)
	m.Lock()
	if open.parser != nil {
		t.Fatal("ticker did not release the closed document of a removed folder")
	}
}
