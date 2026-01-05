package workspace

import (
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"
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

func (m *manager) RemoveWorkspace(uri string) {
	url, err := url.Parse(uri)
	if err != nil {
		slog.Error("Workspace uri parse error: " + err.Error())
		return
	}
	for i, ws := range m.workspaces {
		if ws.Root() != url.Path {
			continue
		}
		ws.hit()
		if !ws.isAlive() {
			m.workspaces = slices.Delete(m.workspaces, i, i+1)
		}
		return
	}
}

func (m *manager) AddWorkspace(uri string) {
	url, err := url.Parse(uri)
	if err != nil {
		slog.Error("Workspace uri parse error: " + err.Error())
		return
	}
	for _, ws := range m.workspaces {
		if ws.Root() == url.Path {
			ws.heal()
			return
		}
	}
	ws := newWs(url.Path, m.mu)
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
