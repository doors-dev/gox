package workspace

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
)

func newWs(root string) *workspace {
	fs := osfs.New(root)
	pats, err := gitignore.ReadPatterns(fs, nil)
	if err != nil {
		slog.Error("gitignore read error: " + err.Error())
	}
	w := &workspace{
		ignore: gitignore.NewMatcher(pats),
		dirs:   make(map[string]*dir),
		root:   root,
	}
	w.scan(root)
	go w.ticker()
	return w
}

type workspace struct {
	ignore gitignore.Matcher
	dirs   map[string]*dir
	root   string
	mu     sync.Mutex
}

func (w *workspace) Load(file File) Doc {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.load(file)
}

func (w *workspace) load(file File) Doc {
	dirPath := file.Dir()
	d, found := w.dirs[dirPath]
	if !found {
		w.scan(dirPath)
		d, found = w.dirs[dirPath]
		if !found {
			return nil
		}
	}
	return d.load(file)
}

func (w *workspace) ticker() {
	for {
		<-time.After(250 * time.Millisecond)
		for name, d := range w.dirs {
			w.mu.Lock()
			d.ScanDeletions()
			if d.IsEmpty() {
				slog.Info("removing dir: " + name)
				delete(w.dirs, name)
			}
			w.mu.Unlock()
		}
	}
}

func (w *workspace) lock() {
	w.mu.Lock()
}

func (w *workspace) unlock() {
	w.mu.Unlock()
}

func (w *workspace) Root() string {
	return w.root
}

func (w *workspace) Ignore() gitignore.Matcher {
	return w.ignore
}

func (w *workspace) scan(path string) {
	_, ok := w.dirs[path]
	if ok {
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	files := make(map[string]File)
	for _, e := range entries {
		path := filepath.Join(path, e.Name())
		rel, _ := filepath.Rel(w.root, path)
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if w.ignore.Match(parts, e.IsDir()) {
			continue
		}
		if e.IsDir() {
			w.scan(path)
			continue
		}
		file, ok := NewFile(path)
		if !ok {
			continue
		}
		_, registered := files[file.Name()]
		if file.Kind() == KindSource || !registered {
			files[file.Name()] = file
			continue
		}
	}
	d := &dir{
		path: path,
		docs: make(map[string]Doc),
		sus:  make(map[string]struct{}),
		ws:   w,
	}
	for _, file := range files {
		_ = d.load(file)
	}
	if d.IsEmpty() {
		return
	}
	w.addDir(d)
}

func (w *workspace) addDir(dir *dir) {
	w.dirs[dir.Path()] = dir
}
