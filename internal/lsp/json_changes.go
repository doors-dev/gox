package lsp

import (
	"errors"

	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/walker"
	"github.com/doors-dev/gox/internal/workspace"
)

var jsonChanges jsonChangesDriver

type jsonChangesDriver struct{}

type ContentChange struct {
	Range common.Range
	Text  string
}

func (r jsonChangesDriver) convertCodeAction(enc common.Encoding, doc workspace.Doc, j Json) (err error) {
	if doc != nil && j.Get("diagnostics").Exists() {
		jsonPos.convertDiagnosticsToSource(enc, doc, j)
		/*
			kind, err := node.Get("kind").String()
			if err != nil {
				return errors.New("code action kind not found")
			}
			if strings.HasPrefix(kind, "refactor.extract") {
				continue
			} */
	}
	edit := j.Get("edit")
	if edit.Exists() {
		err = jsonChanges.convertEdit(enc, edit)
	}
	return
}

func (r jsonChangesDriver) convertCodeActions(enc common.Encoding, doc workspace.Doc, j Json) error {
	arr, err := j.ArrayUseNode()
	if err != nil {
		return errors.New("code actions not found")
	}
	var newActions = make([]ast.Node, 0, len(arr))
	for _, node := range arr {
		err := r.convertCodeAction(enc, doc, &node)
		if err != nil {
			continue
		}
		newActions = append(newActions, node)
	}
	newNode := ast.NewArray(newActions)
	*j = newNode
	return nil
}

func (r jsonChangesDriver) setUpdate(j Json, text string) {
	val := ast.NewPair("text", ast.NewString(text))
	node := ast.NewObject([]ast.Pair{val})
	arr := ast.NewArray([]ast.Node{node})
	_, err := j.Set("contentChanges", arr)
	if err != nil {
		panic("contentChanges set error " + err.Error())
	}
}

func (r jsonChangesDriver) convertEdit(enc common.Encoding, j Json) (err error) {
	changes := j.Get("changes")
	if changes.Exists() {
		newChanges := ast.NewObject(nil)
		changes.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			if path.Index == -1 {
				err = errors.New("wrong format")
				return false
			}
			if path.Key == nil {
				err = errors.New("wrong format")
				return false
			}
			uri := *path.Key
			doc, kind := man.Doc(uri)
			if kind == walker.KindUnknown {
				newChanges.Set(uri, *node)
				return true
			}
			if kind == walker.KindSource {
				err = errors.New("source can't be edited by the server")
				return false
			}
			err = doc.Lock()
			if err != nil {
				return false
			}
			err = jsonPos.convertRangeToSource(enc, doc, node, workspace.Strict)
			doc.Unlock()
			if err != nil {
				return false
			}
			newChanges.Set(doc.SourceFile().URI(), *node)
			return true
		})
		_, err = j.Set("changes", newChanges)
		if err != nil {
			return
		}
	}
	changes = j.Get("documentChanges")
	if !changes.Exists() {
		return
	}
	changesArr, err := changes.ArrayUseNode()
	if err != nil {
		return
	}
	newChanges := make([]ast.Node, 0, len(changesArr))
	for _, node := range changesArr {
		textDoc := node.Get("textDocument")
		if textDoc.Exists() {
			var doc workspace.Doc
			var kind walker.FileKind
			doc, kind, err = jsonDoc.get(&node)
			if err != nil {
				return
			}
			if kind == walker.KindUnknown {
				newChanges = append(newChanges, node)
				continue
			}
			if kind == walker.KindSource {
				err = errors.New("source can't be edited by the server")
				return
			}
			edits := node.Get("edits")
			err = doc.Lock()
			if err != nil {
				return
			}
			err = jsonPos.convertAllToSource(enc, doc, edits, workspace.Strict)
			doc.Unlock()
			if err != nil {
				return
			}
			jsonDoc.setAsSource(&node, doc)
			newChanges = append(newChanges, node)
			continue
		}
		kindNode := node.Get("kind")
		var kind string
		kind, err = kindNode.String()
		if err != nil {
			return
		}
		switch kind {
		case "create":
			newChanges = append(newChanges, node)
		case "rename":
			newChanges = append(newChanges, node)
		case "delete":
			newChanges = append(newChanges, node)
		default:
			err = errors.New("unknown document change kind")
			return
		}
	}
	j.Set("documentChanges", ast.NewArray(newChanges))
	return
}

func (r jsonChangesDriver) getChanges(j Json) ([]ContentChange, error) {
	node := j.Get("contentChanges")
	if !node.Exists() {
		return nil, errors.New("contentChanges field not found")
	}
	arr, _ := node.ArrayUseNode()
	changes := make([]ContentChange, 0, len(arr))
	for _, node := range arr {
		ran, err := jsonPos.getRange(&node)
		if err != nil {
			return nil, err
		}
		text := r.getText(&node)
		changes = append(changes, ContentChange{
			Range: ran,
			Text:  text,
		})
	}
	return changes, nil
}

func (r jsonChangesDriver) getText(j Json) string {
	text := j.Get("text")
	if !text.Exists() {
		return ""
	}
	str, err := text.String()
	if err != nil {
		return ""
	}
	return str
}
