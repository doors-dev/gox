package lsp

import (
	"log/slog"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/jsonrpc"
	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
	"github.com/doors-dev/gox/internal/workspace"
)

func initClientCalls(on func(h onCall, m ...method)) {

	on(func(c caller, j Json) {
		err := jsonInit.setEncodings(j)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		/*
			d, _ := j.MarshalJSON()
			slog.Info("initialize", "req", string(d))
		*/
		uris, err := jsonInit.getWorkspaceDirs(j)
		for _, uri := range uris {
			c.session().addWorkspace(uri)
		}
		c.proxy(j, func(res Json) {
			enc, err := jsonInit.readEncoding(res)
			if err != nil {
				slog.Error("read encoding error: " + err.Error())
				c.err(common.FromErr(jsonrpc2.ErrInternal, err))
				return
			}
			c.session().setEnc(enc)
			err = jsonInit.insertCompletionTriggers(res)
			if err != nil {
				c.err(common.FromErr(jsonrpc2.ErrInternal, err))
				return
			}
			d, _ := res.MarshalJSON()
			slog.Info("initialize", "res", string(d))
			c.res(res)
		})
	}, initialize)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
		err = jsonPos.convertPosTryRangeToTarget(c.enc(), doc, j, workspace.Strict)
		if err != nil {
			pos, err := jsonPos.getPos(j)
			if err != nil {
				c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
				return
			}
			message, ran, ok := doc.Hover(c.enc(), pos)
			if !ok {
				c.res(jsonGenerator.newNull())
			} else {
				c.res(jsonGenerator.newHover(ran, message))
			}
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
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
		err = jsonPos.convertPosTryRangeToTarget(c.enc(), doc, j, workspace.Edge)
		if err != nil {
			pos, err := jsonPos.getPos(j)
			if err != nil {
				c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
				return
			}
			cop, complete := doc.Completions(c.enc(), pos)
			c.res(jsonGenerator.newCompletions(cop, complete))
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertAllToSource(c.enc(), doc, res, workspace.Strict)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert completion response to source"))
				return
			}
			c.res(res)
		})
	}, completion)

	on(func(c caller, j Json) {
		_, kind, err := jsonDoc.get(j)
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
		doc, kind, err := jsonDoc.get(j)
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
		err = jsonPos.convertPosToTarget(c.enc(), doc, j, workspace.Strict)
		if err != nil {
			c.err(common.NewErr(jsonrpc2.ErrInvalidParams, "nothing to rename"))
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertRangeToSource(c.enc(), doc, res, workspace.Strict)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert rename response to source"))
				return
			}
			c.res(res)
		})
	}, prepareRename)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			jsonDoc.setAsTarget(j, doc)
			err = jsonPos.convertPosToTarget(c.enc(), doc, j, workspace.Strict)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInvalidParams, "Nothing to rename"))
				return
			}
		}
		c.proxy(j, func(res Json) {
			err := jsonChanges.convertEdit(c.enc(), res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert edit response"))
				return
			}
			c.res(res)
		})
	}, rename)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			err := jsonPos.convertPosTryRangeToTarget(c.enc(), doc, j, workspace.Edge)
			if err != nil {
				c.res(jsonGenerator.newNull())
				return
			}
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			err = jsonPos.convertLocations(c.enc(), doc, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert locations response"))
				return
			}
			c.res(res)
		})
	}, references, definition, typeDefinition, implementation, prepareTypeHierarchy)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			err = jsonPos.convertRangeToTarget(c.enc(), doc, j, workspace.Approximate)
			if err != nil {
				c.res(jsonGenerator.newEmptyArray())
				return
			}
			context := j.Get("context")
			if context.Exists() {
				jsonPos.convertDiagnosticsToTarget(c.enc(), doc, context)
			}
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			if kind != workspace.KindSource {
				doc = nil
			}
			err := jsonChanges.convertCodeActions(c.enc(), doc, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert code actions response"))
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
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert code action response"))
				return
			}
			c.res(res)
		})
	}, resolveCodeAction)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
			jsonPos.convertAllToSource(c.enc(), doc, res, workspace.Edge)
			c.res(res)
		})
	}, codeLens)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind == workspace.KindSource {
			if doc.Err() != nil {
				c.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
				return
			}
			jsonPos.convertAllToTarget(c.enc(), doc, j, workspace.Approximate)
			jsonDoc.setAsTarget(j, doc)
		}
		c.proxy(j, func(res Json) {
			err := jsonPos.convertCalls(c.enc(), res)
			if err != nil {
				slog.Error("convert calls error: " + err.Error())
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert calls response"))
				return
			}
			c.res(res)
		})
	}, incomingCalls, outgoingCalls, prepareCallHierarchy)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
		err = jsonPos.convertAllToTarget(c.enc(), doc, j, workspace.Edge)
		if err != nil {
			c.res(jsonGenerator.newEmptyArray())
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err = jsonPos.convertAllToSource(c.enc(), doc, res, workspace.Approximate)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert document highlight response"))
				return
			}
			c.res(res)
		})
	}, documentHighlight)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
			err := jsonPos.convertDiagnosticsToSource(c.enc(), doc, j)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert diagnostics response"))
				return
			}
			c.res(res)
		})
	}, diagnostic)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
			err := jsonPos.convertLocations(c.enc(), doc, j)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert document link response"))
				return
			}
			c.res(res)
		})
	}, documentLink)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
		ss := doc.Symbols(c.enc())
		c.res(jsonGenerator.newSymbols(ss))
	}, documentSymbol)

	on(func(c caller, j Json) {
		_, kind, err := jsonDoc.get(j)
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind != workspace.KindSource {
			c.forward()
			return
		}
		c.err(common.NewErr(jsonrpc.ErrUnknown, "folding range not supported for gox, rely on tree sitter"))
	}, foldingRange)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
		formatted, err := doc.Format(c.enc())
		if err != nil {
			c.err(common.FromErr(jsonrpc2.ErrUnknown, err))
			return
		}
		c.res(jsonGenerator.newTextEdits(formatted.Range, formatted.Text))
	}, formatting)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
			err := jsonChanges.convertInlayHints(c.enc(), doc, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert inlay hints response"))
				return
			}
			c.res(res)
		})
	}, inlayHint)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
		err = jsonPos.convertPositionsToTarget(c.enc(), doc, j, workspace.Strict)
		if err != nil {
			c.res(jsonGenerator.newEmptyArray())
			return
		}
		jsonDoc.setAsTarget(j, doc)
		c.proxy(j, func(res Json) {
			err := jsonPos.convertAllToSource(c.enc(), doc, res, workspace.Approximate)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert selection response"))
				return
			}
			c.res(res)
		})
	}, selectionRange)

	on(func(c caller, j Json) {
		doc, kind, err := jsonDoc.get(j)
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
		err = jsonPos.convertPosTryRangeToTarget(c.enc(), doc, j, workspace.Strict)
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
		c.proxy(j, func(res Json) {
			err := jsonPos.convertLocations(c.enc(), nil, res)
			if err != nil {
				c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert locations response"))
				return
			}
			c.res(res)
		})
	}, subtypes, supertypes, symbol)


}
