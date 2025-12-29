package lsp

import (
	"log/slog"

	"github.com/doors-dev/gox/internal/common"
	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
)

func initServerCalls(on func(on onCall, m ...method)) {
	on(func(c caller, j Json) {
		edit := j.Get("edit")
		if !edit.Exists() {
			c.forward()
			return
		}
		err := jsonChanges.convertEdit(c.enc(), edit)
		if err != nil {
			c.err(common.NewErr(jsonrpc2.ErrInternal, "Can't convert edit"))
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
				slog.Error("get workspace folders error: " + err.Error())
				c.res(res)
				return
			}
			c.session().ensureWorkspaces(workspace)
			c.res(res)
		})
	}, workspaceFolders)
}
