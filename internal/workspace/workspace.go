package workspace

import (
	"log/slog"
	"sync"
	"time"

	"github.com/doors-dev/gox/internal/walker"
)

type Unlock func()

type Workspace interface {
	Scan(rootPath string)
	Load(file walker.File) Doc
}

func NewWorkspace() Workspace {
	ws := &workspace{
		dirs: make(map[string]*dir),
	}
	go ws.ticker()
	return ws
}

type workspace struct {
	mu   sync.Mutex
	dirs map[string]*dir
}

func (w *workspace) lock() {
	w.mu.Lock()
}

func (w *workspace) unlock() {
	w.mu.Unlock()
}

func (w *workspace) load(file walker.File, scan bool) Doc {
	dirPath := file.Dir()
	dr, found := w.dirs[dirPath]
	if !found {
		dr = newDir(dirPath, w)
		w.dirs[dirPath] = dr
		slog.Info("watching dir: " + dirPath)
		if scan {
			dr.scan()
		}
	}
	d := dr.load(file)
	if dr.isEmpty() {
		delete(w.dirs, dirPath)
	}
	return d
}

func (w *workspace) Load(file walker.File) Doc {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.load(file, true)
}

func (w *workspace) Scan(rootPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for file := range walker.Walk(rootPath, true) {
		slog.Info("file added: " + file.Path())
		_ = w.load(file, false)
	}
}

func (w *workspace) ticker() {
	for {
		<-time.After(10 * time.Millisecond)
		w.mu.Lock()
		for name, dr := range w.dirs {
			dr.tick()
			if dr.isEmpty() {
				delete(w.dirs, name)
			}
		}
		w.mu.Unlock()
	}
}
