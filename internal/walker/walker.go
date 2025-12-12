package walker

import (
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func Walk(root string, recursive bool) <-chan File {
	ch := make(chan File, 128)
	go func() {
		fs := osfs.New(root)
		pats, err := gitignore.ReadPatterns(fs, nil)
		if err != nil {
			slog.Error("gitignore read error: " + err.Error())
		}
		w := walker{
			root:   root,
			ingore: gitignore.NewMatcher(pats),
			ch:     ch,
			recursive: recursive,
		}
		w.walk(root)
		close(ch)
	}()
	return ch
}

type walker struct {
	root   string
	ingore gitignore.Matcher
	ch     chan<- File
	recursive bool
}

func (w *walker) walk(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("Dir read error: " + err.Error())
		return
	}
	files := make(map[string]File)
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		rel, _ := filepath.Rel(w.root, path)
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if w.ingore.Match(parts, e.IsDir()) {
			continue
		}
		if w.recursive && e.IsDir() {
			w.walk(path)
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
	for _, file := range files {
		w.ch <- file
	}
}
