package lsp

import (
	"github.com/doors-dev/gox/internal/workspace"
)

func initServerNotifs(sess *session, on func(on onNotif, m ...method)) {
	on(func(n notifier, j Json) {
		doc, kind, err := jsonDoc.get(sess.man(), j, false)
		if err != nil {
			return
		}
		if kind == workspace.KindUnknown {
			n.forward()
			return
		}
		if kind == workspace.KindSource {
			return
		}
		if doc.Err() != nil {
			return
		}
		doc.StoreDiag(j)
		if doc.TargetIsOpened() {
			n.notify(doc.GetDiag())
		}
		jsonDoc.setAsSource(j, doc)
		jsonPos.convertDiagnosticsToSource(sess.man(), sess.enc(), doc, j)
		n.notify(j)
	}, publishDiagnostics)
}
