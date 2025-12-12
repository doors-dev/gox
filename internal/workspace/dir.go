package workspace

import (
	"github.com/doors-dev/gox/internal/walker"
)

func newDir(path string, ws ws) *dir {
	return &dir{
		ws:   ws,
		path: path,
		docs: make(map[string]Doc),
		hp:   make(map[string]int),
	}
}

type dir struct {
	ws   ws
	path string
	docs map[string]Doc
	hp   map[string]int
}

const defaultHP = 10

func (d *dir) tick() {
	for name, doc := range d.docs {
		if doc.SourceFile().Exists() {
			d.hp[name] = defaultHP
			continue
		}
		d.hp[name] -= 1
		if d.hp[name] == 0 {
			doc.Delete()
			delete(d.docs, name)
			delete(d.hp, name)
		}
	}
}

func (dr *dir) isEmpty() bool {
	return len(dr.docs) == 0
}

func (dr *dir) load(file walker.File) Doc {
	d, found := dr.docs[file.Name()]
	if !found {
		d = NewDoc(file, dr.ws)
		d.Init()
		if d.Err() == nil {
			dr.docs[d.Name()] = d
			dr.hp[d.Name()] = defaultHP
		}
	}
	return d
}

func (dr *dir) scan() {
	ch := walker.Walk(dr.path, false)
	for file := range ch {
		if !file.IsValid() {
			continue
		}
		_ = dr.load(file)
	}
}
