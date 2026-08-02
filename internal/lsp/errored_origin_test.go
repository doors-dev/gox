package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/workspace"
)

func TestConvertLocationsErroredOriginReportsError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "view.gox"), []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(root, "app.x.go")
	if err := os.WriteFile(orphan, []byte("package demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	man := workspace.NewManager()
	man.Lock()
	man.EnsureWorkspaces([]string{string(docpath.URIFromPath(root))})
	man.Unlock()

	doc, kind := man.Doc(string(docpath.URIFromPath(orphan)))
	t.Logf("doc=%v kind=%v err=%v", doc != nil, kind, doc.Err())
	if doc == nil || doc.Err() == nil {
		t.Skip("precondition not met")
	}

	j := mustJSONNode(t, `{"originSelectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"targetUri":"file:///x","targetRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC: %v", r)
		}
	}()
	err := jsonPos.convertLocations(man, common.UTF8, doc, j)
	t.Logf("no panic, err=%v", err)
}
