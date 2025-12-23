package lsp

import (
	"github.com/doors-dev/gox/internal/workspace"
)

func initServerNotifs(on func(on onNotif, m ...method)) {
	on(func(n notifier, j Json) {
		doc, kind, err := jsonDoc.get(j)
		if err != nil {
			n.err(err)
			return
		}
		if kind == workspace.KindUnknown {
			n.forward()
			return
		}
		if kind == workspace.KindSource {
			panic("source file diagnostics is not expected")
		}
		jsonDoc.setAsSource(j, doc)
		jsonPos.convertDiagnosticsToSource(n.enc(), doc, j)
		n.notify(j)
	}, publishDiagnostics)
}
