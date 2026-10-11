package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/workspace"
)

func TestDocModelBufferCreatedFromBlankLinesTracksBufferText(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "view.gox")
	r := newDocModelRig(t, root)

	r.open(source, "gox", 1, "\n\n")
	r.changeRange(source, 2, 2, 0, "package demo\n\nfunc Marker() {}\n")
	buffer := "\n\npackage demo\n\nfunc Marker() {}\n"

	file, ok := workspace.NewFileFromURI(docModelURI(source))
	if !ok {
		t.Fatal("bad uri")
	}
	reference := workspace.NewDoc(file)
	reference.InitUnsaved()
	if _, err := reference.SourceUpdate(buffer); err != nil {
		t.Fatal(err)
	}
	defer reference.Close()

	man := r.router.session.man()
	man.Lock()
	doc, _ := man.Doc(docModelURI(source))
	var got string
	if doc != nil && doc.Err() == nil {
		got = doc.TargetContent()
	}
	man.Unlock()
	if got != reference.TargetContent() {
		t.Fatalf("document drifted from the editor buffer.\nwant target (from buffer text %q):\n%s\ngot target:\n%s\nclient messages: %v", buffer, reference.TargetContent(), got, r.clientErrors())
	}
}

func TestDocModelOnDiskBlankLinesFileTracksBufferText(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "view.gox")
	docModelWrite(t, source, "\n\n")
	r := newDocModelRig(t, root)

	r.open(source, "gox", 1, "\n\n")
	r.changeRange(source, 2, 2, 0, "package demo\n\nfunc Marker() {}\n")
	buffer := "\n\npackage demo\n\nfunc Marker() {}\n"

	file, _ := workspace.NewFileFromURI(docModelURI(filepath.Join(t.TempDir(), "view.gox")))
	reference := workspace.NewDoc(file)
	reference.InitUnsaved()
	if _, err := reference.SourceUpdate(buffer); err != nil {
		t.Fatal(err)
	}
	defer reference.Close()

	man := r.router.session.man()
	man.Lock()
	doc, _ := man.Doc(docModelURI(source))
	got := doc.TargetContent()
	man.Unlock()
	strip := func(s string) string {
		lines := strings.Split(s, "\n")
		kept := lines[:0]
		for _, line := range lines {
			if strings.Contains(line, "view.gox") {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	}
	if strip(got) != strip(reference.TargetContent()) {
		t.Fatalf("on-disk document drifted.\nwant:\n%s\ngot:\n%s", reference.TargetContent(), got)
	}
}
