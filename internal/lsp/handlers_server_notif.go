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
			panic("source file diagnostics is not expected")
		}
		if doc.Err() != nil {
			return
		}
		jsonDoc.setAsSource(j, doc)
		jsonPos.convertDiagnosticsToSource(sess.man(), sess.enc(), doc, j)
		n.notify(j)
	}, publishDiagnostics)
}
