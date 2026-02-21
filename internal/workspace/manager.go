package workspace

import (
	"log/slog"
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
	m.Lock()
	defer m.Unlock()
	toRemove := make([]string, 0)
	pathes := make([]string, 0, len(uris))
	for _, uri := range uris {
		docuri, err := docpath.ParseDocumentURI(uri)
		if err != nil {
			slog.Error("Workspace uri parse error: " + err.Error())
			continue
		}
		pathes = append(pathes, docuri.Path())
	}
	for _, ws := range m.workspaces {
		i := slices.Index(pathes, ws.Root())
		if i == -1 {
			toRemove = append(toRemove, ws.Root())
		} else {
			pathes = slices.Delete(pathes, i, i+1)
		}
	}
	for _, path := range toRemove {
		for i, ws := range m.workspaces {
			if ws.Root() != path {
				continue
			}
			m.workspaces = slices.Delete(m.workspaces, i, i+1)
			return
		}
	}
	for _, path := range pathes {
		ws := newWs(path, m.mu)
		m.workspaces = append(m.workspaces, ws)
	}
}

func (m *manager) RemoveWorkspace(uri string) {
	docuri, err := docpath.ParseDocumentURI(uri)
	if err != nil {
		slog.Error("Workspace uri parse error: " + err.Error())
		return
	}
	for i, ws := range m.workspaces {
		if ws.Root() != docuri.Path() {
			continue
		}
		m.workspaces = slices.Delete(m.workspaces, i, i+1)
		return
	}
}

func (m *manager) AddWorkspace(uri string) {
	docuri, err := docpath.ParseDocumentURI(uri)
	if err != nil {
		slog.Error("Workspace uri parse error: " + err.Error())
		return
	}
	for _, ws := range m.workspaces {
		if ws.Root() == docuri.Path() {
			return
		}
	}
	ws := newWs(docuri.Path(), m.mu)
	m.workspaces = append(m.workspaces, ws)
}

func (m *manager) Doc(uri string) (Doc, FileKind) {
	file, ok := NewFileFromURI(uri)
	if !ok {
		return nil, KindUnknown
	}
	for _, ws := range m.workspaces {
		if strings.HasPrefix(file.Path(), ws.Root()) {
			doc := ws.Load(file)
			return doc, file.Kind()
		}
	}
	return nil, file.Kind()
}
