package lsp

import (
	"github.com/doors-dev/gox/internal/common"
	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
	"github.com/doors-dev/gox/internal/workspace"
)

func initClientCalls(sess *session, on func(h onCall, m ...method)) {

	on(func(c caller, j Json) {
		err := jsonInit.setEncodings(j)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		uris, err := jsonInit.getWorkspaceDirs(j)
		for _, uri := range uris {
			sess.addWorkspace(uri)
		}
		isVscode := jsonInit.isVscodeExtension(j)
		c.proxy(j, func(res Json) {
			if isVscode {
				jsonInit.prepeareForVscodeExtension(res)
			}
			enc, err := jsonInit.readEncoding(res)
			if err != nil {
				c.err(common.FromErr(jsonrpc2.ErrInternal, err))
				return
			}
			sess.setEnc(enc)
			err = jsonInit.insertCompletionTriggers(res)
			if err != nil {
				c.err(common.FromErr(jsonrpc2.ErrInternal, err))
				return
			}
			c.res(res)
		})
	}, initialize)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		err = jsonPos.convertPosTryRangeToTarget(sess.enc(), doc, j, workspace.Strict)
		if err != nil {
			pos, err := jsonPos.getPos(j)
			if err != nil {
				c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
				return
			}
			message, ran, ok := doc.Hover(sess.enc(), pos)
			if !ok {
				c.res(jsonGenerator.newNull())
			} else {
				c.res(jsonGenerator.newHover(ran, message))
			}
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertRangeToSource(sess.enc(), doc, res, workspace.Approximate)
			if err != nil {
				c.res(nil)
				return
			}
			c.res(res)
		})
	}, hover)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		err = jsonPos.convertPosTryRangeToTarget(sess.enc(), doc, j, workspace.Edge)
		if err != nil {
			pos, err := jsonPos.getPos(j)
			if err != nil {
				c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
				return
			}
			cop, complete := doc.Completions(sess.enc(), pos)
			c.res(jsonGenerator.newCompletions(cop, complete))
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			jsonChanges.convertCompletions(sess.enc(), doc, res)
			c.res(res)
		})
	}, completion)

	on(func(c caller, j Json) {
		_, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		c.res(jsonGenerator.newNoSemanticTokens())
	}, semanticTokensFull, semanticTokensRange)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		err = jsonPos.convertPosToTarget(sess.enc(), doc, j, workspace.Strict)
		if err != nil {
			c.err(common.NewErr(jsonrpc2.ErrInvalidParams, "Nothing to rename here."))
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertRangeToSource(sess.enc(), doc, res, workspace.Strict)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not map the rename response back to the source file: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, prepareRename)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			jsonDoc.setAsTarget(j, doc)
			err = jsonPos.convertPosToTarget(sess.enc(), doc, j, workspace.Strict)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInvalidParams, "Nothing to rename here."))
				return
			}
		}
		c.proxy(j, func(res Json) {
			err := jsonChanges.convertEdit(sess.man(), sess.enc(), res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the edit response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, rename)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			err := jsonPos.convertPosTryRangeToTarget(sess.enc(), doc, j, workspace.Edge)
			if err != nil {
				c.res(jsonGenerator.newNull())
				return
			}
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			err = jsonPos.convertLocations(sess.man(), sess.enc(), doc, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the locations response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, references, definition, typeDefinition, implementation, prepareTypeHierarchy)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			err = jsonPos.convertRangeToTarget(sess.enc(), doc, j, workspace.Edge)
			if err != nil {
				c.res(jsonGenerator.newEmptyArray())
				return
			}
			context := j.Get("context")
			if context.Exists() {
				jsonPos.convertDiagnosticsToTarget(sess.man(), sess.enc(), doc, context)
			}
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			if kind != workspace.KindSource {
				doc = nil
			}
			err := jsonChanges.convertCodeActions(sess.man(), sess.enc(), doc, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the code actions response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, codeAction)

	on(func(c caller, j Json) {
		j.Unset("diagnostics")
		err := jsonChanges.convertCodeAction(sess.man(), sess.enc(), nil, j)
		if err != nil {
			c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the code action: "+err.Error()))
			return
		}
		c.proxy(j, func(res Json) {
			err := jsonChanges.convertCodeAction(sess.man(), sess.enc(), nil, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the code action response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, resolveCodeAction)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			jsonPos.convertAllToSource(sess.enc(), doc, res, workspace.Edge)
			c.res(res)
		})
	}, codeLens)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			jsonPos.convertAllToTarget(sess.enc(), doc, j, workspace.Approximate)
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			err := jsonPos.convertCalls(sess.man(), sess.enc(), res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the call hierarchy response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, incomingCalls, prepareCallHierarchy)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		origin := doc
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			jsonPos.convertAllToTarget(sess.enc(), doc, j, workspace.Approximate)
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			err := jsonPos.convertOutgoingCalls(sess.man(), sess.enc(), origin, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the call hierarchy response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, outgoingCalls)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		err = jsonPos.convertAllToTarget(sess.enc(), doc, j, workspace.Edge)
		if err != nil {
			c.res(jsonGenerator.newEmptyArray())
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err = jsonPos.convertAllToSource(sess.enc(), doc, res, workspace.Approximate)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the document highlight response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, documentHighlight)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertDiagnosticsToSource(sess.man(), sess.enc(), doc, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the diagnostics response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, diagnostic)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertLocations(sess.man(), sess.enc(), doc, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the document link response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, documentLink)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		ss := doc.Symbols(sess.enc())
		c.res(jsonGenerator.newSymbols(ss))
	}, documentSymbol)

	on(func(c caller, j Json) {
		_, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		c.res(jsonGenerator.newEmptyArray())
	}, foldingRange)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		formatted, err := doc.Format(sess.enc())
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, err))
			return
		}
		c.res(jsonGenerator.newTextEdits(formatted.Range, formatted.Text))
	}, formatting)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		if err = jsonPos.convertRangeToTarget(sess.enc(), doc, j, workspace.Edge); err != nil {
			c.res(jsonGenerator.newEmptyArray())
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonChanges.convertInlayHints(sess.enc(), doc, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the inlay hints response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, inlayHint)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		err = jsonPos.convertPositionsToTarget(sess.enc(), doc, j, workspace.Strict)
		if err != nil {
			c.res(jsonGenerator.newEmptyArray())
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertAllToSource(sess.enc(), doc, res, workspace.Approximate)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the selection range response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, selectionRange)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		if doc.Err() != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		err = jsonPos.convertPosTryRangeToTarget(sess.enc(), doc, j, workspace.Strict)
		if err != nil {
			c.res(jsonGenerator.newEmptyArray())
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			c.res(res)
		})
	}, signatureHelp)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind == workspace.KindSource {
			err = jsonPos.convertItemRanges(sess.enc(), doc, j, convertToTarget, workspace.Edge)
			if err != nil {
				c.res(jsonGenerator.newNull())
				return
			}
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			err := jsonPos.convertLocations(sess.man(), sess.enc(), nil, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the locations response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, subtypes, supertypes)

	on(func(c caller, j Json) {
		c.proxy(j, func(res Json) {
			err := jsonPos.convertLocations(sess.man(), sess.enc(), nil, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the locations response: "+err.Error()))
				return
			}
			c.res(res)
		})
	}, symbol)

}
