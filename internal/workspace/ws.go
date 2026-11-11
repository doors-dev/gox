package workspace

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

func newWs(root string, dirs map[string]*dir) *workspace {
	fs := osfs.New(root)
	pats, err := gitignore.ReadPatterns(fs, nil)
	if err != nil {
		slog.Error("Error reading .gitignore: " + err.Error())
	}
	w := &workspace{
		ignore: gitignore.NewMatcher(pats),
		dirs:   dirs,
		root:   root,
	}
	w.scan(root)
	return w
}

type workspace struct {
	ignore gitignore.Matcher
	dirs   map[string]*dir
	root   string
}

func (w *workspace) Load(file File) Doc {
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

func (w *workspace) Open(file File) Doc {
	if file.Kind() != KindSource || file.Exists() {
		return w.load(file)
	}
	path := file.Dir()
	d, found := w.dirs[path]
	if !found {
		w.scan(path)
		d, found = w.dirs[path]
	}
	if !found {
		d = newDir(path)
		w.addDir(d)
	}
	return d.open(file)
}

func (w *workspace) Root() string {
	return w.root
}

func (w *workspace) scan(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	files := make(map[string]File)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
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
	d, found := w.dirs[path]
	if !found {
		d = newDir(path)
	}
	for _, file := range files {
		_ = d.load(file)
	}
	if !found && !d.IsEmpty() {
		w.addDir(d)
	}
}

func (w *workspace) addDir(dir *dir) {
	w.dirs[dir.Path()] = dir
}
