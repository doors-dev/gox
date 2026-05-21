package lsp

import (
	"github.com/doors-dev/gox/internal/common"
	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
)

func initServerCalls(sess *session, on func(on onCall, m ...method)) {
	on(func(c caller, j Json) {
		edit := j.Get("edit")
		if !edit.Exists() {
			c.forward()
			return
		}
		err := jsonChanges.convertEdit(sess.man(), sess.enc(), edit)
		if err != nil {
			c.err(common.NewErr(jsonrpc2.ErrInternal, "Could not convert the workspace edit."))
			return
		}
		c.proxy(j, func(res Json) {
			c.res(res)
		})
	}, applyEdit)

	on(func(c caller, j Json) {
		c.proxy(j, func(res Json) {
			workspace, err := jsonInit.getWorkspaceDirsFromArray(res)
			if err != nil {
				c.res(res)
				return
			}
			sess.ensureWorkspacesLocked(workspace)
			c.res(res)
		})
	}, workspaceFolders)
}
