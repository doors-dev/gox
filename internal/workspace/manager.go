package workspace

import (
	"log/slog"
	"net/url"
	"strings"

	"github.com/doors-dev/gox/internal/walker"
)

type Manager = *manager

func NewManager() Manager {
	return &manager{
		workspaces: make(map[string]Workspace),
		virtual:    NewWorkspace(),
	}
}

type manager struct {
	workspaces map[string]Workspace
	virtual    Workspace
}


func (m *manager) AddWorkspace(uri string) {
	url, err := url.Parse(uri)
	if err != nil {
		slog.Error("parse error: " + err.Error())
		return
	}
	ws := NewWorkspace()
	m.workspaces[url.Path] = ws
	ws.Scan(url.Path)
}

func (m *manager) Doc(uri string) (Doc, walker.FileKind) {
	file, ok := walker.NewFileFromURI(uri)
	if !ok {
		return nil, walker.KindUnknown
	}
	for rootPath, ws := range m.workspaces {
		if strings.HasPrefix(file.Path(), rootPath) {
			doc := ws.Load(file)
			return doc, file.Kind()
		}
	}
	doc := m.virtual.Load(file)
	return doc, file.Kind()
}
