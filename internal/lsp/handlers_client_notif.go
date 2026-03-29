package lsp

import (
	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
	"github.com/doors-dev/gox/internal/workspace"
)

func initClientNotifs(sess *session, on func(on onNotif, m ...method)) {
	on(func(n notifier, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			n.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind == workspace.KindUnknown {
			n.forward()
			return
		}
		version, err := jsonDoc.getVersion(j)
		if err != nil {
			n.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		text, err := jsonDoc.getText(j)
		if err != nil {
			n.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if doc.Err() != nil {
			n.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		switch kind {
		case workspace.KindSource:
			doc.SourceOpen(int32(version))
			doc.SourceUpdate(text)
			if doc.TargetIsOpened() {
				return
			}
			jsonDoc.setAsTarget(j, doc)
			n.notify(j)
		case workspace.KindTarget:
			doc.TargetOpen(int32(version))
			if !doc.SourceIsOpened() {
				jsonDoc.setVersion(j, 0)
				n.notify(j)
				return
			}
			sess.showInfo("This generated .x.go file may be outdated until the .gox source file is saved.")
		}
	}, didOpen)

	on(func(n notifier, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, true)
		if err != nil {
			n.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind == workspace.KindUnknown {
			n.forward()
			return
		}
		if doc == nil && kind == workspace.KindTarget {
			sess.showWarn("This generated file does not belong to the current workspace.")
			n.forward()
			return
		}
		if doc.Err() != nil {
			n.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		switch kind {
		case workspace.KindSource:
			changes, err := jsonChanges.getChanges(j)
			if err != nil {
				n.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
				return
			}
			updated := false
			for _, change := range changes {
				var upd bool
				if change.Range.IsValid() {
					upd, err = doc.SourcePatch(sess.enc(), change.Range, change.Text)
					updated = updated || upd
				} else {
					upd, err = doc.SourceUpdate(change.Text)
					updated = updated || upd
				}
				if err != nil {
					break
				}
			}
			if err != nil {
				n.err(common.FromErr(jsonrpc2.ErrInternal, err))
				return
			}
			if updated {
				doc.Assemble()
			}
			jsonDoc.setAsTarget(j, doc)
			jsonChanges.setUpdate(j, doc.TargetContent())
			n.notify(j)
			if updated {
				pos, do := doc.GoxImportPos(sess.enc())
				if !do {
					return
				}
				edit := jsonGenerator.newInsertEdit(
					doc.SourceFile().URI(),
					pos,
					"\n\nimport \"github.com/doors-dev/gox\"",
					"GoX imported",
				)
				sess.callClient(
					applyEdit,
					edit,
				)
			}
		case workspace.KindTarget:
			n.err(common.NewErr(jsonrpc2.ErrUnknown, "Generated file edits are reverted on save. Edit the .gox source, or rename .x.go to .go to edit this file directly."))
			return
		}
	}, didChange)

	on(func(n notifier, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, true)
		if err != nil {
			n.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if doc == nil && kind == workspace.KindTarget {
			sess.showWarn("This generated file does not belong to the current workspace.")
			n.forward()
			return
		}
		if kind == workspace.KindUnknown {
			n.forward()
			return
		}
		if kind == workspace.KindTarget {
			needUpd, err := doc.CheckTarget()
			if err != nil {
				n.err(common.FromErr(jsonrpc2.ErrInternal, err))
				return
			}
			if !needUpd {
				n.forward()
				return
			}
			if err := doc.TargetWrite(); err != nil {
				n.err(common.FromErr(jsonrpc2.ErrInternal, err))
				return
			}
			sess.showWarn("Generated file changes were reverted on save. Edit the .gox source, or rename .x.go to .go to edit this file directly.")
			text := j.Get("text")
			if text.Exists() {
				j.Set("text", ast.NewString(doc.TargetContent()))
			}
			n.notify(j)
			return
		}
		if doc.Err() != nil {
			n.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		err = doc.Save()
		if err != nil {
			n.err(common.FromErr(jsonrpc2.ErrInternal, err))
			return
		}
		jsonDoc.setAsTarget(j, doc)
		n.notify(j)
	}, didSave)

	on(func(n notifier, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			n.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		if kind == workspace.KindUnknown {
			n.forward()
			return
		}
		if doc.Err() != nil {
			n.err(common.FromErr(jsonrpc2.ErrUnknown, doc.Err()))
			return
		}
		switch kind {
		case workspace.KindSource:
			doc.SourceClose()
			if doc.TargetIsOpened() {
				return
			}
			jsonDoc.setAsTarget(j, doc)
			n.notify(j)
		case workspace.KindTarget:
			doc.TargetClose()
			if doc.SourceIsOpened() {
				return
			}
			n.forward()
		}
	}, didClose)

	on(func(n notifier, j Json) {
		added, removed, err := jsonInit.getWorkspaceChanges(j)
		if err != nil {
			n.err(common.FromErr(jsonrpc2.ErrInvalidParams, err))
			return
		}
		for _, uri := range added {
			sess.addWorkspace(uri)
		}
		for _, uri := range removed {
			sess.removeWorkspace(uri)
		}
		n.forward()
	}, didChangeWorkspaceFolders)

}
