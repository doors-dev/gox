package lsp

import (
	"errors"
	"slices"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/workspace"
)

var jsonPos jsonPosDriver

type jsonPosDriver struct{}

func (r jsonPosDriver) convertPosTryRangeToTarget(enc common.Encoding, doc workspace.Doc, j Json, mode workspace.ConvMode) error {
	err := r.convertPosToTarget(enc, doc, j, mode)
	if err != nil {
		return err
	}
	err = r.convertRangeToTarget(enc, doc, j, mode)
	if err != nil {
		r.unsetRange(j)
	}
	return nil
}

func (r jsonPosDriver) setRange(j Json, ran common.Range) {
	_, err := j.Set("range", r.fromRange(ran))
	if err != nil {
		panic("range set error " + err.Error())
	}
}

func (r jsonPosDriver) getRange(j Json) (common.Range, error) {
	ran := j.Get("range")
	if !ran.Exists() {
		return common.NoRange(), errors.New("The range field is missing.")
	}
	return r.intoRange(ran)
}

func (r jsonPosDriver) getPos(j Json) (common.Pos, error) {
	pos := j.Get("position")
	if !pos.Exists() {
		return common.NoPos(), errors.New("The position field is missing.")
	}
	return r.intoPos(pos)
}

func (r jsonPosDriver) setPos(j Json, pos common.Pos) {
	_, err := j.Set("position", r.fromPos(pos))
	if err != nil {
		panic("position set error " + err.Error())
	}
}

func (r jsonPosDriver) unsetRange(j Json) {
	j.Unset("range")
}

func (r jsonPosDriver) fromRange(ran common.Range) ast.Node {
	startPos := r.fromPos(ran.Beg())
	endPos := r.fromPos(ran.End())
	start := ast.NewPair("start", startPos)
	end := ast.NewPair("end", endPos)
	return ast.NewObject([]ast.Pair{start, end})
}

func (r jsonPosDriver) intoRange(j Json) (common.Range, error) {
	start := j.Get("start")
	end := j.Get("end")
	if !start.Exists() || !end.Exists() {
		return common.NoRange(), errors.New("The range is missing required fields.")
	}
	startPos, err := r.intoPos(start)
	if err != nil {
		return common.NoRange(), err
	}
	endPos, err := r.intoPos(end)
	if err != nil {
		return common.NoRange(), err
	}
	return common.NewRange(startPos, endPos), nil
}

func (r jsonPosDriver) intoPos(j Json) (common.Pos, error) {
	line := j.Get("line")
	character := j.Get("character")
	if !line.Exists() || !character.Exists() {
		return common.NoPos(), errors.New("The position is missing required fields.")
	}
	lineNum, err := line.Int64()
	if err != nil {
		return common.NoPos(), err
	}
	characterNum, err := character.Int64()
	if err != nil {
		return common.NoPos(), err
	}
	return common.NewPos(int(lineNum), int(characterNum)), nil
}

func (r jsonPosDriver) fromPos(pos common.Pos) ast.Node {
	line := ast.NewPair("line", ast.NewAny(pos.Line()))
	col := ast.NewPair("character", ast.NewAny(pos.Column()))
	return ast.NewObject([]ast.Pair{line, col})
}

type convertTo int

const (
	convertToSource convertTo = iota
	convertToTarget
)

func (r jsonPosDriver) convertLocations(man workspace.Manager, enc common.Encoding, origin workspace.Doc, j Json) (err error) {
	if j.TypeSafe() == ast.V_NULL {
		return nil
	}
	if j.TypeSafe() == ast.V_ARRAY {
		j.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			err = jsonPos.convertLocations(man, enc, origin, node)
			if err != nil {
				return false
			}
			return true
		})
		return
	}
	if origin != nil {
		originRange := j.Get("originSelectionRange")
		if originRange.Exists() {
			err = origin.Err()
			if err != nil {
				return
			}
			var ran common.Range
			ran, err = jsonPos.intoRange(originRange)
			if err != nil {
				return
			}
			newRange, ok := origin.SourceRange(enc, ran, workspace.Approximate)
			if !ok {
				return errors.New("The range could not be mapped.")
			}
			_, err = j.Set("originSelectionRange", jsonPos.fromRange(newRange))
			if err != nil {
				return
			}
		}
	}
	location := j.Get("location")
	if location.Exists() {
		err = r.convertLocations(man, enc, origin, location)
		if err != nil {
			return
		}
	}
	var doc workspace.Doc
	if j.Get("uri").Exists() {
		var kind workspace.FileKind
		doc, kind, err = jsonDoc.get(man, j, false)
		if err != nil {
			return
		}
		if kind == workspace.KindUnknown {
			return
		}
		if kind == workspace.KindSource {
			return errors.New("The server cannot reference source files here.")
		}
		jsonDoc.setAsSource(j, doc)
	} else {
		doc = origin
	}
	if doc == nil {
		return
	}
	err = doc.Err()
	if err != nil {
		return
	}
	for _, key := range []string{"range", "targetRange", "targetSelectionRange", "selectionRange"} {
		node := j.Get(key)
		if !node.Exists() {
			continue
		}
		var ran common.Range
		ran, err = jsonPos.intoRange(node)
		if err != nil {
			return
		}
		newRange, ok := doc.SourceRange(enc, ran, workspace.Approximate)
		if !ok {
			return errors.New("The range could not be mapped.")
		}
		j.Set(key, jsonPos.fromRange(newRange))
	}
	return
}

func (r jsonPosDriver) convertCalls(man workspace.Manager, enc common.Encoding, j Json) (err error) {
	j.ForEach(func(path ast.Sequence, node *ast.Node) bool {
		if path.Index == -1 {
			return false
		}
		var doc workspace.Doc
		var kind workspace.FileKind
		doc, kind, err = jsonDoc.get(man, node, false)
		if err != nil {
			return false
		}
		if kind == workspace.KindUnknown {
			return true
		}
		if kind == workspace.KindSource {
			err = errors.New("A source file is not expected here.")
			return false
		}
		err = doc.Err()
		if err != nil {
			return false
		}
		err = jsonPos.convertAllToSource(enc, doc, node, workspace.Approximate)
		jsonDoc.setAsSource(node, doc)
		return true
	})
	return
}

func (r jsonPosDriver) convertOutgoingCalls(man workspace.Manager, enc common.Encoding, origin workspace.Doc, j Json) (err error) {
	j.ForEach(func(path ast.Sequence, node *ast.Node) bool {
		if path.Index == -1 {
			return false
		}
		to := node.Get("to")
		if to.Exists() {
			var doc workspace.Doc
			var kind workspace.FileKind
			doc, kind, err = jsonDoc.get(man, to, false)
			if err != nil {
				return false
			}
			if kind != workspace.KindUnknown {
				if kind == workspace.KindSource {
					err = errors.New("A source file is not expected here.")
					return false
				}
				err = doc.Err()
				if err != nil {
					return false
				}
				err = jsonPos.convertAllToSource(enc, doc, to, workspace.Approximate)
				if err != nil {
					return false
				}
				jsonDoc.setAsSource(to, doc)
			}
		}
		if origin == nil {
			return true
		}
		err = origin.Err()
		if err != nil {
			return false
		}
		fromRanges := node.Get("fromRanges")
		if !fromRanges.Exists() {
			return true
		}
		fromRanges.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			if path.Index == -1 {
				return false
			}
			err = jsonPos.convertRangeInPlace(enc, origin, node, convertToSource, workspace.Approximate)
			return err == nil
		})
		return err == nil
	})
	return
}

var sourceDeniedDiagnosticCodes = []string{
	"QF1003",
}

var sourceDeniedDiagnosticMessagePrefixes = []string{
	"could use tagged switch",
}

func (r jsonPosDriver) diagnosticDeniedOnSource(j Json) bool {
	if code, err := j.Get("code").String(); err == nil {
		if slices.Contains(sourceDeniedDiagnosticCodes, code) {
			return true
		}
	}
	if message, err := j.Get("message").String(); err == nil {
		for _, prefix := range sourceDeniedDiagnosticMessagePrefixes {
			if strings.HasPrefix(message, prefix) {
				return true
			}
		}
	}
	return false
}

func (r jsonPosDriver) convertDiagnosticsToTarget(man workspace.Manager, enc common.Encoding, doc workspace.Doc, j Json) error {
	return r.convertDiagnostics(man, enc, doc, j, convertToTarget)
}

func (r jsonPosDriver) convertDiagnosticsToSource(man workspace.Manager, enc common.Encoding, doc workspace.Doc, j Json) error {
	return r.convertDiagnostics(man, enc, doc, j, convertToSource)
}

func (r jsonPosDriver) convertDiagnostics(man workspace.Manager, enc common.Encoding, doc workspace.Doc, j Json, direction convertTo) error {
	key := "diagnostics"
	var diagnostics = j.Get(key)
	if !diagnostics.Exists() {
		key = "items"
		diagnostics = j.Get(key)
	}
	if diagnostics.Exists() {
		diag, err := diagnostics.ArrayUseNode()
		if err != nil {
			return errors.New("Diagnostics must be an array.")
		}
		newDiag := make([]ast.Node, 0, len(diag))
		for _, node := range diag {
			if direction == convertToSource && r.diagnosticDeniedOnSource(&node) {
				continue
			}
			ran, err := jsonPos.getRange(&node)
			if err != nil {
				err = errors.New("A diagnostic range is missing.")
				continue
			}
			var newRan common.Range
			var ok bool
			switch direction {
			case convertToSource:
				newRan, ok = doc.SourceRange(enc, ran, workspace.Approximate)
			case convertToTarget:
				newRan, ok = doc.TargetRange(enc, ran, workspace.Approximate)
			default:
				panic("Unexpected")
			}
			if !ok {
				continue
			}
			jsonPos.setRange(&node, newRan)
			newDiag = append(newDiag, node)
		}
		j.Set(key, ast.NewArray(newDiag))
	}
	related := j.Get("relatedDocuments")
	if !related.Exists() {
		return nil
	}
	var err error
	newRelated := ast.NewObject(nil)
	related.ForEach(func(path ast.Sequence, node *ast.Node) bool {
		if path.Index == -1 {
			return false
		}
		if path.Key == nil {
			err = errors.New("The related document format is invalid.")
			return false
		}
		uri := *path.Key
		doc, kind := man.Doc(uri)
		if kind == workspace.KindUnknown {
			newRelated.Set(uri, *node)
			return true
		}
		if kind == workspace.KindSource {
			err = errors.New("Diagnostics are not expected for a source file.")
			return false
		}
		if doc == nil {
			newRelated.Set(uri, *node)
			return true
		}
		err = doc.Err()
		if err != nil {
			return false
		}
		err = r.convertDiagnostics(man, enc, doc, node, direction)
		if err != nil {
			return false
		}
		newRelated.Set(doc.SourceFile().URI(), *node)
		return true
	})
	if err != nil {
		return err
	}
	_, err = j.Set("relatedDocuments", newRelated)
	if err != nil {
		panic("relatedDocuments set error " + err.Error())
	}
	return nil
}

func (r jsonPosDriver) convertRangeToSource(enc common.Encoding, doc workspace.Doc, j Json, mode workspace.ConvMode) error {
	return r.convertRange(enc, doc, j, convertToSource, mode)
}

func (r jsonPosDriver) convertRangeToTarget(enc common.Encoding, doc workspace.Doc, j Json, mode workspace.ConvMode) error {
	return r.convertRange(enc, doc, j, convertToTarget, mode)
}

func (r jsonPosDriver) convertItemRanges(enc common.Encoding, doc workspace.Doc, j Json, direction convertTo, mode workspace.ConvMode) (err error) {
	item := j.Get("item")
	if !item.Exists() {
		return errors.New("there is no item")
	}
	for _, key := range []string{"range", "selectionRange"} {
		node := item.Get(key)
		if !node.Exists() {
			continue
		}
		var ran common.Range
		ran, err = jsonPos.intoRange(node)
		if err != nil {
			return
		}
		var newRan common.Range
		var ok bool
		switch direction {
		case convertToSource:
			newRan, ok = doc.SourceRange(enc, ran, mode)
		case convertToTarget:
			newRan, ok = doc.TargetRange(enc, ran, mode)
		default:
			panic("Unexpected")
		}
		if !ok {
			return errors.New("The range could not be mapped.")
		}
		item.Set(key, jsonPos.fromRange(newRan))
		if !ok {
			return errors.New("The range could not be mapped.")
		}
	}
	return nil
}

func (r jsonPosDriver) convertRange(enc common.Encoding, doc workspace.Doc, j Json, direction convertTo, mode workspace.ConvMode) error {
	ran, err := jsonPos.getRange(j)
	if err != nil {
		return err
	}
	var newRan common.Range
	var ok bool
	switch direction {
	case convertToSource:
		newRan, ok = doc.SourceRange(enc, ran, mode)
	case convertToTarget:
		newRan, ok = doc.TargetRange(enc, ran, mode)
	default:
		panic("Unexpected")
	}
	if !ok {
		return errors.New("The range could not be mapped.")
	}
	jsonPos.setRange(j, newRan)
	return nil
}

func (r jsonPosDriver) convertPositionsToTarget(enc common.Encoding, doc workspace.Doc, j Json, mode workspace.ConvMode) error {
	node := j.Get("positions")
	if !node.Exists() {
		return errors.New("The positions field is missing.")
	}
	positions, err := node.ArrayUseNode()
	if err != nil {
		return errors.New("Positions must be an array.")
	}
	newPositions := make([]ast.Node, 0, len(positions))
	for i := range positions {
		pos, err := r.intoPos(&positions[i])
		if err != nil {
			return err
		}
		newPos, ok := doc.TargetPos(enc, pos, mode)
		if !ok {
			return errors.New("The position could not be mapped.")
		}
		newPositions = append(newPositions, r.fromPos(newPos))
	}
	j.Set("positions", ast.NewArray(newPositions))
	return nil
}

func (r jsonPosDriver) convertPosToSource(enc common.Encoding, doc workspace.Doc, j Json, mode workspace.ConvMode) error {
	return r.convertPos(enc, doc, j, convertToSource, mode)
}

func (r jsonPosDriver) convertPosToTarget(enc common.Encoding, doc workspace.Doc, j Json, mode workspace.ConvMode) error {
	return r.convertPos(enc, doc, j, convertToTarget, mode)
}

func (r jsonPosDriver) convertPos(enc common.Encoding, doc workspace.Doc, j Json, direction convertTo, mode workspace.ConvMode) error {
	pos, err := jsonPos.getPos(j)
	if err != nil {
		return err
	}
	var newPos common.Pos
	var ok bool
	switch direction {
	case convertToSource:
		newPos, ok = doc.SourcePos(enc, pos, mode)
	case convertToTarget:
		newPos, ok = doc.TargetPos(enc, pos, mode)
	default:
		panic("Unexpected")
	}
	if !ok {
		return errors.New("The position could not be mapped.")
	}
	jsonPos.setPos(j, newPos)
	return nil
}

func (r jsonPosDriver) convertAllToSource(enc common.Encoding, doc workspace.Doc, j Json, mode workspace.ConvMode) (err error) {
	return r.convertAll(enc, doc, j, convertToSource, mode)
}

func (r jsonPosDriver) convertAllToTarget(enc common.Encoding, doc workspace.Doc, j Json, mode workspace.ConvMode) (err error) {
	return r.convertAll(enc, doc, j, convertToTarget, mode)
}

func (r jsonPosDriver) convertRangeInPlace(enc common.Encoding, doc workspace.Doc, j Json, direction convertTo, mode workspace.ConvMode) error {
	ran, err := jsonPos.intoRange(j)
	if err != nil {
		return err
	}
	var newRan common.Range
	var ok bool
	switch direction {
	case convertToSource:
		newRan, ok = doc.SourceRange(enc, ran, mode)
	case convertToTarget:
		newRan, ok = doc.TargetRange(enc, ran, mode)
	default:
		panic("Unexpected")
	}
	if !ok {
		return errors.New("The range could not be mapped.")
	}
	j.Set("start", jsonPos.fromPos(newRan.Beg()))
	j.Set("end", jsonPos.fromPos(newRan.End()))
	return nil
}

func (r jsonPosDriver) convertAll(enc common.Encoding, doc workspace.Doc, j Json, direction convertTo, mode workspace.ConvMode) (err error) {
	j.ForEach(func(path ast.Sequence, node *ast.Node) bool {
		if path.Index == -1 {
			return false
		}
		if path.Key == nil {
			err = r.convertAll(enc, doc, node, direction, mode)
			return err == nil
		}
		switch *path.Key {
		case "fromRanges":
			node.ForEach(func(path ast.Sequence, node *ast.Node) bool {
				if path.Index == -1 {
					return false
				}
				err = r.convertRangeInPlace(enc, doc, node, direction, mode)
				return err == nil
			})
			if err != nil {
				return false
			}
		case "range", "insert", "replace", "selectionRange":
			err = r.convertRangeInPlace(enc, doc, node, direction, mode)
			return err == nil
		case "position":
			var pos common.Pos
			pos, err = jsonPos.intoPos(node)
			if err != nil {
				return false
			}
			var newPos common.Pos
			var ok bool
			switch direction {
			case convertToSource:
				newPos, ok = doc.SourcePos(enc, pos, mode)
			case convertToTarget:
				newPos, ok = doc.TargetPos(enc, pos, mode)
			default:
				panic("Unexpected")
			}
			if !ok {
				err = errors.New("The position could not be mapped.")
				return false
			}
			node.SetAny("line", newPos.Line())
			node.SetAny("character", newPos.Column())
		default:
			err = r.convertAll(enc, doc, node, direction, mode)
			return err == nil
		}
		return true
	})
	return
}
