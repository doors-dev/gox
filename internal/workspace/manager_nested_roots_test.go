package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/doors-dev/gox/internal/docpath"
)

func nestedRootsTree(t *testing.T) (parent, child, source string) {
	t.Helper()
	parent = t.TempDir()
	child = filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(child, "view.gox")
	if err := os.WriteFile(source, []byte(sampleSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return parent, child, source
}

func rootURI(path string) string {
	return string(docpath.URIFromPath(path))
}

func TestAddWorkspaceSkipsNestedRoot(t *testing.T) {
	parent, child, source := nestedRootsTree(t)
	m := NewManager()
	defer m.StopAll()
	m.AddWorkspace(rootURI(parent))
	m.AddWorkspace(rootURI(child))
	if roots := m.roots(); len(roots) != 1 || roots[0] != parent {
		t.Fatalf("roots = %v, want [%s]", roots, parent)
	}
	if doc, kind := m.Doc(rootURI(source)); doc == nil || kind != KindSource {
		t.Fatalf("Doc(source) = (%v, %v), want source doc", doc, kind)
	}
}

func TestAddWorkspaceRemovesCoveredChildren(t *testing.T) {
	parent, child, _ := nestedRootsTree(t)
	m := NewManager()
	defer m.StopAll()
	m.AddWorkspace(rootURI(child))
	covered := append([]*workspace(nil), m.workspaces...)
	m.AddWorkspace(rootURI(parent))
	if roots := m.roots(); len(roots) != 1 || roots[0] != parent {
		t.Fatalf("roots = %v, want [%s]", roots, parent)
	}
	for _, ws := range covered {
		select {
		case <-ws.done:
		default:
			t.Fatalf("covered workspace %s was not stopped", ws.Root())
		}
	}
}

func TestEnsureWorkspacesPrunesNestedRoots(t *testing.T) {
	parent, child, _ := nestedRootsTree(t)
	m := NewManager()
	defer m.StopAll()
	m.EnsureWorkspaces([]string{rootURI(child), rootURI(parent)})
	if roots := m.roots(); len(roots) != 1 || roots[0] != parent {
		t.Fatalf("roots = %v, want [%s]", roots, parent)
	}
}

func TestEnsureWorkspacesReplacesChildWithParent(t *testing.T) {
	parent, child, _ := nestedRootsTree(t)
	m := NewManager()
	defer m.StopAll()
	m.EnsureWorkspaces([]string{rootURI(child)})
	covered := append([]*workspace(nil), m.workspaces...)
	m.EnsureWorkspaces([]string{rootURI(parent), rootURI(child)})
	if roots := m.roots(); len(roots) != 1 || roots[0] != parent {
		t.Fatalf("roots = %v, want [%s]", roots, parent)
	}
	for _, ws := range covered {
		select {
		case <-ws.done:
		default:
			t.Fatalf("covered workspace %s was not stopped", ws.Root())
		}
	}
}
