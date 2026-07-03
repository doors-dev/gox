package lsp

import (
	"errors"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/workspace"
)

var jsonChanges jsonChangesDriver

type jsonChangesDriver struct{}

type ContentChange struct {
	Range    common.Range
	HasRange bool
	Text     string
}

func (r jsonChangesDriver) convertCompletions(enc common.Encoding, doc workspace.Doc, j Json) {
	defaults := j.Get("itemDefaults")
	if defaults.Exists() {
		err := jsonPos.convertAllToSource(enc, doc, defaults, workspace.Strict)
		if err != nil {
			j.Set("items", *jsonGenerator.newEmptyArray())
			return
		}
	}
	items := j.Get("items")
	if !items.Exists() {
		return
	}
	arr, err := items.ArrayUseNode()
	if err != nil {
		return
	}
	var newArr = make([]ast.Node, 0, len(arr))
	for _, node := range arr {
		err := jsonPos.convertAllToSource(enc, doc, &node, workspace.Strict)
		if err != nil {
			continue
		}
		newArr = append(newArr, node)
	}
	newNode := ast.NewArray(newArr)
	j.Set("items", newNode)
}

func (r jsonChangesDriver) convertInlayHints(enc common.Encoding, doc workspace.Doc, j Json) (err error) {
	if j.TypeSafe() == ast.V_NULL {
		return nil
	}
	j.ForEach(func(path ast.Sequence, node *ast.Node) bool {
		if path.Index == -1 || path.Key != nil {
			err = errors.New("The gopls inlay hints response format is invalid.")
			return false
		}
		if err != nil {
			return false
		}
		err = jsonPos.convertPosToTarget(enc, doc, node, workspace.Edge)
		if err != nil {
			return false
		}
		textEdits := node.Get("textEdits")
		if !textEdits.Exists() {
			return true
		}
		textEdits.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			if path.Index == -1 || path.Key != nil {
				err = errors.New("The gopls inlay hints response format is invalid.")
				return false
			}
			err = jsonPos.convertRangeToTarget(enc, doc, node, workspace.Strict)
			return err == nil
		})
		return err == nil
	})
	return
}

func (r jsonChangesDriver) convertCodeAction(man workspace.Manager, enc common.Encoding, doc workspace.Doc, j Json) (err error) {
	if doc != nil {
		if kindNode := j.Get("kind"); kindNode.Exists() {
			if kind, e := kindNode.String(); e == nil && strings.HasPrefix(kind, "refactor.extract") {
				return errors.New("gox: extract action handled natively")
			}
		}
		if j.Get("diagnostics").Exists() {
			jsonPos.convertDiagnosticsToSource(man, enc, doc, j)
		}
	}
	edit := j.Get("edit")
	if edit.Exists() {
		err = jsonChanges.convertEdit(man, enc, edit)
	}
	return
}

func (r jsonChangesDriver) convertCodeActions(man workspace.Manager, enc common.Encoding, doc workspace.Doc, j Json) error {
	if j.TypeSafe() == ast.V_NULL {
		return nil
	}
	arr, err := j.ArrayUseNode()
	if err != nil {
		return errors.New("Code actions are missing or invalid.")
	}
	var newActions = make([]ast.Node, 0, len(arr))
	for _, node := range arr {
		err := r.convertCodeAction(man, enc, doc, &node)
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

func (r jsonChangesDriver) convertEdit(man workspace.Manager, enc common.Encoding, j Json) (err error) {
	changes := j.Get("changes")
	if changes.Exists() {
		newChanges := ast.NewObject(nil)
		changes.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			if path.Index == -1 {
				err = errors.New("Could not read the workspace edit.")
				return false
			}
			if path.Key == nil {
				err = errors.New("Could not read the workspace edit.")
				return false
			}
			uri := *path.Key
			doc, kind := man.Doc(uri)
			if kind == workspace.KindUnknown {
				newChanges.Set(uri, *node)
				return true
			}
			err = doc.Err()
			if err != nil {
				return false
			}
			if kind == workspace.KindSource {
				err = jsonPos.convertRangeToTarget(enc, doc, node, workspace.Strict)
				newChanges.Set(doc.TargetFile().URI(), *node)
			} else {
				err = jsonPos.convertRangeToSource(enc, doc, node, workspace.Strict)
				newChanges.Set(doc.SourceFile().URI(), *node)
			}
			if err != nil {
				return false
			}
			return true
		})
		_, jerr := j.Set("changes", newChanges)
		if jerr != nil {
			err = errors.New("Could not update the workspace changes.")
			return
		}
	}
	changes = j.Get("documentChanges")
	if !changes.Exists() {
		return
	}
	changesArr, jerr := changes.ArrayUseNode()
	if jerr != nil {
		err = errors.New("Could not read the document changes.")
		return
	}
	newChanges := make([]ast.Node, 0, len(changesArr))
	for _, node := range changesArr {
		textDoc := node.Get("textDocument")
		if textDoc.Exists() {
			var doc workspace.Doc
			var kind workspace.FileKind
			doc, kind, err = jsonDoc.get(man, &node, true)
			if err != nil {
				return
			}
			if kind == workspace.KindUnknown {
				newChanges = append(newChanges, node)
				continue
			}
			err = doc.Err()
			if err != nil {
				return
			}
			edits := node.Get("edits")
			if kind == workspace.KindSource {
				err = jsonPos.convertAllToTarget(enc, doc, edits, workspace.Strict)
				jsonDoc.setAsTarget(&node, doc)
			} else {
				err = jsonPos.convertAllToSource(enc, doc, edits, workspace.Strict)
				jsonDoc.setAsSource(&node, doc)
			}
			if err != nil {
				return
			}
			newChanges = append(newChanges, node)
			continue
		}
		kindNode := node.Get("kind")
		var kind string
		kind, jerr = kindNode.String()
		if err != nil {
			err = errors.New("Could not read the workspace edit.")
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
			err = errors.New("The document change kind is not supported.")
			return
		}
	}
	j.Set("documentChanges", ast.NewArray(newChanges))
	return
}

func (r jsonChangesDriver) getChanges(j Json) ([]ContentChange, error) {
	node := j.Get("contentChanges")
	if !node.Exists() {
		return nil, errors.New("Could not read the content changes.")
	}
	arr, _ := node.ArrayUseNode()
	changes := make([]ContentChange, 0, len(arr))
	for _, node := range arr {
		ran := common.NoRange()
		hasRange := node.Get("range").Exists()
		if hasRange {
			var err error
			ran, err = jsonPos.getRange(&node)
			if err != nil {
				return nil, err
			}
			if !ran.IsValid() {
				return nil, errors.New("The edit range is invalid.")
			}
		}
		text := r.getText(&node)
		changes = append(changes, ContentChange{
			Range:    ran,
			HasRange: hasRange,
			Text:     text,
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
