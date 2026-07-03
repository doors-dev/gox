package workspace

import (
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/doors-dev/gox/internal/docpath"
)

type Manager = *manager

func NewManager() Manager {
	return &manager{
		mu: &sync.Mutex{},
	}

}

type manager struct {
	mu         *sync.Mutex
	workspaces []*workspace
}

func (m *manager) Lock() {
	m.mu.Lock()
}

func (m *manager) Unlock() {
	m.mu.Unlock()
}

func (m *manager) EnsureWorkspaces(uris []string) {
	toRemove := make([]string, 0)
	paths := make([]string, 0, len(uris))
	for _, uri := range uris {
		path, err := workspacePath(uri)
		if err != nil {
			slog.Error("Workspace uri parse error: " + err.Error())
			continue
		}
		if slices.Contains(paths, path) {
			continue
		}
		paths = append(paths, path)
	}
	for _, ws := range m.workspaces {
		i := slices.Index(paths, ws.Root())
		if i == -1 {
			toRemove = append(toRemove, ws.Root())
		} else {
			paths = slices.Delete(paths, i, i+1)
		}
	}
	for _, path := range toRemove {
		for i, ws := range m.workspaces {
			if ws.Root() != path {
				continue
			}
			ws.Stop()
			m.workspaces = slices.Delete(m.workspaces, i, i+1)
			break
		}
	}
	for _, path := range paths {
		ws := newWs(path, m.mu)
		m.workspaces = append(m.workspaces, ws)
	}
	if len(paths) != 0 || len(toRemove) != 0 {
		slog.Debug("Workspace roots updated", "added", paths, "removed", toRemove, "roots", m.roots())
	}
}

func (m *manager) RemoveWorkspace(uri string) {
	path, err := workspacePath(uri)
	if err != nil {
		slog.Error("Workspace uri parse error: " + err.Error())
		return
	}
	for i, ws := range m.workspaces {
		if ws.Root() != path {
			continue
		}
		ws.Stop()
		m.workspaces = slices.Delete(m.workspaces, i, i+1)
		slog.Debug("Workspace root removed", "root", path, "roots", m.roots())
		return
	}
}

func (m *manager) AddWorkspace(uri string) {
	path, err := workspacePath(uri)
	if err != nil {
		slog.Error("Workspace uri parse error: " + err.Error())
		return
	}
	for _, ws := range m.workspaces {
		if ws.Root() == path {
			return
		}
	}
	ws := newWs(path, m.mu)
	m.workspaces = append(m.workspaces, ws)
	slog.Debug("Workspace root added", "root", path, "roots", m.roots())
}

func (m *manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ws := range m.workspaces {
		ws.Stop()
	}
	m.workspaces = nil
}

func (m *manager) Doc(uri string) (Doc, FileKind) {
	file, ok := NewFileFromURI(uri)
	if !ok {
		return nil, KindUnknown
	}
	for _, ws := range m.workspaces {
		root := ws.Root()
		path := file.Path()
		if containsPath(root, path) {
			doc := ws.Load(file)
			return doc, file.Kind()
		}
	}
	slog.Debug("Document is outside active workspaces", "uri", uri, "path", file.Path(), "kind", file.Kind(), "roots", m.roots())
	return nil, file.Kind()
}

func (m *manager) RootsLocked() []string {
	return m.roots()
}

func (m *manager) roots() []string {
	roots := make([]string, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		roots = append(roots, ws.Root())
	}
	return roots
}

func workspacePath(uri string) (string, error) {
	docuri, err := docpath.ParseDocumentURI(uri)
	if err != nil {
		return "", err
	}
	path := docuri.Path()
	if path == "" {
		return "", errors.New("workspace URI path is empty")
	}
	return filepath.Clean(path), nil
}

func containsPath(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
