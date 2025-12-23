package workspace


type dir struct {
	path string
	docs map[string]Doc
	sus  map[string]struct{}
	ws   *workspace
}

func (d *dir) IsEmpty() bool {
	return len(d.docs) == 0
}

func (dr *dir) load(file File) Doc {
	d, found := dr.docs[file.Name()]
	if !found {
		d = NewDoc(file, dr.ws)
		d.Init()
		if d.Err() == nil {
			dr.docs[d.Name()] = d
		}
	}
	return d
}

func (d *dir) ScanDeletions() {
	for name, doc := range d.docs {
		if !doc.SourceFile().Exists() {
			_, ok := d.sus[name]
			if !ok {
				d.sus[name] = struct{}{}
				continue
			}
			delete(d.sus, name)
			delete(d.docs, name)
			doc.targetRemove()
			continue
		}
		delete(d.sus, name)
		if !doc.TargetFile().Exists() {
			doc.Save()
		}
	}
}

func (d *dir) Path() string {
	return d.path
}

