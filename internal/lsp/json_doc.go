package lsp

import (
	"errors"
	"github.com/doors-dev/gox/internal/workspace"
)

var jsonDoc jsonDocDriver

type jsonDocDriver struct{}

func (r jsonDocDriver) getText(j Json) (string, error) {
	textDoc := j.Get("textDocument")
	if !textDoc.Exists() {
		return "", errors.New("text document not found")
	}
	text := textDoc.Get("text")
	if !text.Exists() {
		return "", errors.New("text field not found")
	}
	str, err := text.String()
	if err != nil {
		return "", errors.New("text field not valid")
	}
	return str, nil
}

func (r jsonDocDriver) get(man workspace.Manager, j Json, forEdit bool) (workspace.Doc, workspace.FileKind, error) {
	uri := func() Json {
		uri := j.Get("uri")
		if uri.Exists() {
			return uri
		}
		textDoc := j.Get("textDocument")
		if textDoc.Exists() {
			uri = textDoc.Get("uri")
			if uri.Exists() {
				return uri
			}
			return nil
		}
		uri = j.Get("targetUri")
		if uri.Exists() {
			return uri
		}
		item := j.Get("item")
		if item.Exists() {
			uri = item.Get("uri")
			if uri.Exists() {
				return uri
			}
		}
		from := j.Get("from")
		if from.Exists() {
			uri = from.Get("uri")
			if uri.Exists() {
				return uri
			}
		}
		return nil
	}()
	if uri == nil {
		return nil, workspace.KindUnknown, errors.New("uri field not found")
	}
	uriStr, err := uri.String()
	if err != nil {
		return nil, workspace.KindUnknown, errors.New("uri field not valid")
	}
	doc, kind := man.Doc(uriStr)
	if !forEdit && doc == nil && kind == workspace.KindTarget {
		return nil, workspace.KindUnknown, nil
	}
	return doc, kind, nil
}

func (r jsonDocDriver) setVersion(j Json, version int32) {
	textDoc := j.Get("textDocument")
	if !textDoc.Exists() {
		return
	}
	_, err := textDoc.SetAny("version", version)
	if err != nil {
		panic("version set error " + err.Error())
	}
}

func (r jsonDocDriver) getVersion(j Json) (int, error) {
	textDoc := j.Get("textDocument")
	if !textDoc.Exists() {
		return 0, errors.New("text document not found")
	}
	version := textDoc.Get("version")
	if !version.Exists() {
		return 0, errors.New("version field not found")
	}
	i, err := version.Int64()
	if err != nil {
		return 0, errors.New("version field not valid")
	}
	return int(i), nil
}

func (r jsonDocDriver) setAsSource(j Json, doc workspace.Doc) {
	r.setAs(j, doc, workspace.KindSource)
}

func (r jsonDocDriver) setAsTarget(j Json, doc workspace.Doc) {
	r.setAs(j, doc, workspace.KindTarget)
}

func (r jsonDocDriver) setAs(j Json, doc workspace.Doc, kind workspace.FileKind) {
	var file workspace.File
	switch kind {
	case workspace.KindSource:
		file = doc.SourceFile()
	case workspace.KindTarget:
		file = doc.TargetFile()
	default:
		panic("invalid kind")
	}
	uri := j.Get("uri")
	if uri.Exists() {
		_, err := j.SetAny("uri", file.URI())
		if err != nil {
			panic("uri set error " + err.Error())
		}
		return
	}
	uri = j.Get("targetUri")
	if uri.Exists() {
		_, err := j.SetAny("targetUri", file.URI())
		if err != nil {
			panic("uri set error " + err.Error())
		}
		return
	}
	item := j.Get("item")
	if item.Exists() {
		uri = item.Get("uri")
		if uri.Exists() {
			_, err := item.SetAny("uri", file.URI())
			if err != nil {
				panic("uri set error " + err.Error())
			}
			return
		}
	}
	from := j.Get("from")
	if from.Exists() {
		uri = from.Get("uri")
		if uri.Exists() {
			_, err := from.SetAny("uri", file.URI())
			if err != nil {
				panic("uri set error " + err.Error())
			}
			return
		}
	}
	textDoc := j.Get("textDocument")
	if !textDoc.Exists() {
		panic("text document or uri not found")
	}
	_, err := textDoc.SetAny("uri", file.URI())
	if err != nil {
		panic("uri set error " + err.Error())
	}
	if kind != workspace.KindTarget {
		return
	}
	if textDoc.Get("languageId").Exists() {
		_, err = textDoc.SetAny("languageId", "go")
		if err != nil {
			panic("language id set error " + err.Error())
		}
	}
	if textDoc.Get("text").Exists() {
		_, err = textDoc.SetAny("text", doc.TargetContent())
		if err != nil {
			panic("text set error " + err.Error())
		}
	}
}
