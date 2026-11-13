package workspace

import (
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/doors-dev/gox/internal/docpath"
)

type Manager = *manager

func NewManager() Manager {
	return &manager{
		mu:   &sync.Mutex{},
		dirs: make(map[string]*dir),
		done: make(chan struct{}),
	}

}

type manager struct {
	mu               *sync.Mutex
	workspaceFolders []string
	workspaces       []*workspace
	dirs             map[string]*dir
	onGhost          func(Doc)
	onReload         func(Doc)
	done             chan struct{}
	startOnce        sync.Once
	stopOnce         sync.Once
}

func (m *manager) OnGhost(f func(Doc)) {
	m.onGhost = f
}

func (m *manager) ghost(doc Doc) {
	if m.onGhost != nil {
		m.onGhost(doc)
	}
}

func (m *manager) OnReload(f func(Doc)) {
	m.onReload = f
}

func (m *manager) Reload(doc Doc) bool {
	if !doc.Reload() {
		return false
	}
	if m.onReload != nil {
		m.onReload(doc)
	}
	return true
}

func (m *manager) Lock() {
	m.mu.Lock()
}

func (m *manager) Unlock() {
	m.mu.Unlock()
}

func (m *manager) EnsureWorkspaces(uris []string) {
	workspaceFolders := make([]string, 0, len(uris))
	for _, uri := range uris {
		path, err := workspacePath(uri)
		if err != nil {
			slog.Error("Workspace uri parse error: " + err.Error())
			continue
		}
		if !slices.Contains(workspaceFolders, path) {
			workspaceFolders = append(workspaceFolders, path)
		}
	}
	m.setWorkspaceFolders(workspaceFolders)
}

func (m *manager) RemoveWorkspace(uri string) {
	path, err := workspacePath(uri)
	if err != nil {
		slog.Error("Workspace uri parse error: " + err.Error())
		return
	}
	workspaceFolders := slices.DeleteFunc(slices.Clone(m.workspaceFolders), func(f string) bool {
		return f == path
	})
	m.setWorkspaceFolders(workspaceFolders)
}

func (m *manager) AddWorkspace(uri string) {
	path, err := workspacePath(uri)
	if err != nil {
		slog.Error("Workspace uri parse error: " + err.Error())
		return
	}
	if slices.Contains(m.workspaceFolders, path) {
		return
	}
	m.setWorkspaceFolders(append(slices.Clone(m.workspaceFolders), path))
}

func (m *manager) setWorkspaceFolders(workspaceFolders []string) {
	m.workspaceFolders = workspaceFolders
	workspaces := make([]*workspace, 0, len(workspaceFolders))
	for _, root := range pruneNestedPaths(workspaceFolders) {
		i := slices.IndexFunc(m.workspaces, func(ws *workspace) bool {
			return ws.Root() == root
		})
		if i == -1 {
			workspaces = append(workspaces, newWs(root, m.dirs))
			continue
		}
		workspaces = append(workspaces, m.workspaces[i])
	}
	m.workspaces = workspaces
	m.release()
	if len(m.workspaces) != 0 {
		m.startOnce.Do(func() {
			go m.ticker()
		})
	}
	slog.Debug("Workspace roots updated", "workspaceFolders", m.workspaceFolders, "roots", m.roots())
}

func (m *manager) release() {
	for path, d := range m.dirs {
		if m.covering(path) == nil {
			d.releaseIdle()
		}
		if d.IsEmpty() {
			delete(m.dirs, path)
		}
	}
}

func (m *manager) ticker() {
	for {
		select {
		case <-time.After(250 * time.Millisecond):
		case <-m.done:
			return
		}
		m.mu.Lock()
		select {
		case <-m.done:
			m.mu.Unlock()
			return
		default:
		}
		m.release()
		for path, d := range m.dirs {
			d.ProcessFileRemovals(m.ghost, m.Reload)
			if d.IsEmpty() {
				delete(m.dirs, path)
			}
		}
		m.mu.Unlock()
	}
}

func (m *manager) covering(path string) *workspace {
	for _, ws := range m.workspaces {
		if containsPath(ws.Root(), path) {
			return ws
		}
	}
	return nil
}

func pruneNestedPaths(paths []string) []string {
	pruned := make([]string, 0, len(paths))
	for _, path := range paths {
		nested := false
		for _, other := range paths {
			if other != path && containsPath(other, path) {
				nested = true
				break
			}
		}
		if !nested {
			pruned = append(pruned, path)
		}
	}
	return pruned
}

func (m *manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopOnce.Do(func() {
		close(m.done)
	})
	for _, d := range m.dirs {
		for _, doc := range d.docs {
			doc.Close()
		}
	}
	clear(m.dirs)
	m.workspaceFolders, m.workspaces = nil, nil
}

func (m *manager) Doc(uri string) (Doc, FileKind) {
	return m.doc(uri, (*workspace).Load)
}

func (m *manager) Open(uri string) (Doc, FileKind) {
	return m.doc(uri, (*workspace).Open)
}

func (m *manager) doc(uri string, load func(*workspace, File) Doc) (Doc, FileKind) {
	file, ok := NewFileFromURI(uri)
	if !ok {
		return nil, KindUnknown
	}
	if d, ok := m.dirs[file.Dir()]; ok {
		if doc, ok := d.docs[file.Name()]; ok && doc.opened() {
			return doc, file.Kind()
		}
	}
	if ws := m.covering(file.Path()); ws != nil {
		return load(ws, file), file.Kind()
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
