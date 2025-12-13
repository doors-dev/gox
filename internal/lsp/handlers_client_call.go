package lsp

import (
	"errors"
	"log/slog"

	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
	"github.com/doors-dev/gox/internal/walker"
	"github.com/doors-dev/gox/internal/workspace"
)

func initClientCalls(on func(h onCall, m ...method)) {

	on(func(c caller, j Json) {
		d, _ := j.MarshalJSON()
		slog.Info("initialize", "res", string(d))
		folders := j.Get("workspaceFolders")
		if folders == nil {
			slog.Error("no workspace folders")
			return
		}
		a, err := folders.ArrayUseNode()
		if err != nil {
			slog.Error("get workspace folders error: " + err.Error())
			return
		}
		for _, folder := range a {
			uri := folder.Get("uri")
			if uri == nil {
				slog.Error("no uri")
				continue
			}
			uriStr, err := uri.String()
			if err != nil {
				slog.Error("get uri error: " + err.Error())
				continue
			}
			slog.Info("adding workspace: " + uriStr)
			man.AddWorkspace(uriStr)
		}
		c.proxy(j, func(res Json) {
			d, _ := res.MarshalJSON()
			slog.Info("initialize", "res", string(d))
			c.res(res)
		})
	}, initialize)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		pos, err := jsonPos.getPos(j)
		if err != nil {
			c.err(err)
			return
		}
		err = doc.Lock()
		if err != nil {
			c.err(doc.Err())
			return
		}
		defer doc.Unlock()
		hoverResult := doc.Hover(c.enc(), pos)
		if !hoverResult.Ok() {
			return
		}
		message, ran, ok := hoverResult.Token()
		if ok {
			c.res(jsonGenerator.newHover(ran, message))
			return
		}
		err = jsonPos.convertRangeToTarget(c.enc(), doc, j, workspace.Approximate)
		if err != nil {
			jsonPos.unsetRange(j)
		}
		jsonDoc.setAsTarget(j, doc)
		newPos, ok := hoverResult.Portal()
		if !ok {
			slog.Error("hover result error")
			c.res(nil)
			return
		}
		jsonPos.setPos(j, newPos)
		c.proxy(j, func(res Json) {
			doc.Lock()
			defer doc.Unlock()
			err := jsonPos.convertRangeToSource(c.enc(), doc, res, workspace.Approximate)
			if err != nil {
				c.res(nil)
				return
			}
			c.res(res)
		})
	}, hover)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		err = doc.Lock()
		if err != nil {
			c.err(doc.Err())
			return
		}
		defer doc.Unlock()
		err = jsonPos.convertRangeToTarget(c.enc(), doc, j, workspace.Strict)
		if err != nil {
			jsonPos.unsetRange(j)
		}
		jsonDoc.setAsTarget(j, doc)
		err = jsonPos.convertPosToTarget(c.enc(), doc, j, workspace.Edge)
		if err != nil {
			c.err(jsonrpc2.ErrInvalidParams)
			return
		}
		c.proxy(j, func(res Json) {
			doc.Lock()
			defer doc.Unlock()
			err := jsonPos.convertAllToSource(c.enc(), doc, res, workspace.Strict)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, completion)

	on(func(c caller, j Json) {
		_, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		c.res(jsonGenerator.newNoSemanticTokens())
	}, semanticTokensFull, semanticTokensRange)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		err = doc.Lock()
		if err != nil {
			c.err(doc.Err())
			return
		}
		defer doc.Unlock()
		jsonDoc.setAsTarget(j, doc)
		err = jsonPos.convertPosToTarget(c.enc(), doc, j, workspace.Strict)
		if err != nil {
			c.err(jsonrpc2.ErrInvalidParams)
			return
		}
		c.proxy(j, func(res Json) {
			doc.Lock()
			defer doc.Unlock()
			err := jsonPos.convertRangeToSource(c.enc(), doc, res, workspace.Strict)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, prepareRename)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if kind == walker.KindSource {
			err = doc.Lock()
			if err != nil {
				c.err(doc.Err())
				return
			}
			defer doc.Unlock()
			jsonDoc.setAsTarget(j, doc)
			err = jsonPos.convertPosToTarget(c.enc(), doc, j, workspace.Strict)
			if err != nil {
				c.err(jsonrpc2.ErrInvalidParams)
				return
			}
		}
		c.proxy(j, func(res Json) {
			err := jsonChanges.convertEdit(c.enc(), res)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, rename)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if kind == walker.KindSource {
			err = doc.Lock()
			if err != nil {
				c.err(doc.Err())
				return
			}
			defer doc.Unlock()
			err := jsonPos.convertPosToTarget(c.enc(), doc, j, workspace.Strict)
			if err != nil {
				c.err(err)
				return
			}
			err = jsonPos.convertRangeToTarget(c.enc(), doc, j, workspace.Strict)
			if err != nil {
				jsonPos.unsetRange(j)
			}
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			err = jsonPos.convertLocations(c.enc(), doc, res)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, references, definition, typeDefinition)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind == walker.KindSource {
			err = doc.Lock()
			if err != nil {
				c.err(doc.Err())
				return
			}
			ran, err := jsonPos.getRange(j)
			if err != nil {
				c.err(err)
				return
			}
			newRan, ok := doc.TargetRange(c.enc(), ran, workspace.Approximate)
			if !ok {
				c.err(jsonrpc2.ErrInvalidParams)
				return
			}
			jsonPos.setRange(j, newRan)
			doc.Unlock()
			context := j.Get("context")
			if context.Exists() {
				jsonPos.convertDiagnosticsToTarget(c.enc(), doc, context)
			}
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			if kind != walker.KindSource {
				doc = nil
			}
			err := jsonChanges.convertCodeActions(c.enc(), doc, res)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, codeAction)

	on(func(c caller, j Json) {
		j.Unset("edit")
		j.Unset("diagnostics")
		c.proxy(j, func(res Json) {
			err := jsonChanges.convertCodeAction(c.enc(), nil, res)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, resolveCodeAction)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		err = doc.Lock()
		if err != nil {
			c.err(doc.Err())
			return
		}
		defer doc.Unlock()
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := doc.Lock()
			if err != nil {
				c.err(doc.Err())
				return
			}
			defer doc.Unlock()
			jsonPos.convertAllToSource(c.enc(), doc, res, workspace.Edge)
			c.res(res)
		})
	}, codeLens)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind == walker.KindSource {
			err := doc.Lock()
			if err != nil {
				c.err(err)
				return
			}
			defer doc.Unlock()
			jsonPos.convertAllToTarget(c.enc(), doc, j, workspace.Approximate)
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			err := jsonPos.convertCalls(c.enc(), res)
			if err != nil {
				slog.Error("convert calls error: " + err.Error())
				c.err(err)
				return
			}
			c.res(res)
		})
	}, incomingCalls, outgoingCalls, prepareCallHierarchy)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		err = doc.Lock()
		if err != nil {
			c.err(doc.Err())
			return
		}
		defer doc.Unlock()
		err = jsonPos.convertAllToTarget(c.enc(), doc, j, workspace.Edge)
		if err != nil {
			c.err(err)
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := doc.Lock()
			if err != nil {
				c.err(err)
				return
			}
			defer doc.Unlock()
			err = jsonPos.convertAllToSource(c.enc(), doc, res, workspace.Approximate)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, documentHighlight)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertDiagnosticsToSource(c.enc(), doc, j)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, diagnostic)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertLocations(c.enc(), doc, j)
			if err != nil {
				c.err(err)
				return
			}
			c.res(res)
		})
	}, documentLink)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		err = doc.Lock()
		if err != nil {
			c.err(err)
			return
		}
		defer doc.Unlock()
		ss := doc.Symbols(c.enc())
		c.res(jsonGenerator.newSymbols(ss))
	}, documentSymbol)

	on(func(c caller, j Json) {
		_, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		c.err(errors.New("folding range not supported for gox, rely on tree sitter"))
	}, foldingRange)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(err)
			return
		}
		if kind != walker.KindSource {
			c.forward()
			return
		}
		formatted, err := doc.Format(c.enc())
		if err != nil {
			c.err(err)
			return
		}
		c.res(jsonGenerator.newTextEdits(formatted.Range, formatted.Text))
	}, formatting)

}
