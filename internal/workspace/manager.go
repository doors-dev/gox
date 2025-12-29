package workspace

import (
	"log/slog"
	"net/url"
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
	mu       *sync.Mutex
	workspaces []*workspace
}

func (m *manager) Lock() {
	m.mu.Lock()
}

func (m *manager) Unlock() {
	m.mu.Unlock()
}

func (m *manager) AddWorkspace(uri string) {
	url, err := url.Parse(uri)
	if err != nil {
		slog.Error("parse error: " + err.Error())
		return
	}
	for _, ws := range m.workspaces {
		if ws.Root() == url.Path {
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
