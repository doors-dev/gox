package lsp

import (
	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/workspace"
	"log/slog"
)

type session struct {
	bridge   Bridge
	encoding common.Encoding
	manager  workspace.Manager
}

func (s *session) man() workspace.Manager {
	return s.manager
}

func (s *session) ensureWorkspaces(uris []string) {
	s.manager.EnsureWorkspaces(uris)
}

func (s *session) addWorkspace(uri string) {
	s.manager.AddWorkspace(uri)
}

func (s *session) removeWorkspace(uri string) {
	s.manager.RemoveWorkspace(uri)
}

func (s *session) enc() common.Encoding {
	return s.encoding
}

func (s *session) setEnc(e common.Encoding) {
	s.encoding = e
}

func (s *session) notifGo(m method, params Json) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	s.bridge.Notify(Gopls, Request{
		Method: string(m),
		Params: data,
	})
}

func (s *session) notifClient(m method, params Json) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	s.bridge.Notify(Client, Request{
		Method: string(m),
		Params: data,
	})
}

func (s *session) callClient(m method, params Json) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}

	s.bridge.Call(Client, Request{
		Method: string(m),
		Params: data,
	}, func(r Response) {
		if r.Err != nil {
			slog.Error("Call client result error", "method", m, "error", r.Err.Error())
			s.logError("Call client result error: [method=" + string(m) + ", error=" + r.Err.Error() + "]")
		}
	})
}

func (s *session) showInfo(msg string) {
	s.show(msg, 3)
}

func (s *session) showWarn(msg string) {
	s.show(msg, 2)
}

func (s *session) showError(msg string) {
	s.show(msg, 1)
}

func (s *session) logError(msg string) {
	s.log(msg, 1)
}

func (s *session) log(msg string, typ int) {
	message := ast.NewPair("message", ast.NewString(msg))
	typNode := ast.NewPair("type", ast.NewAny(typ))
	node := ast.NewObject([]ast.Pair{message, typNode})
	s.notifClient(logMessage, &node)
}

func (s *session) show(msg string, typ int) {
	message := ast.NewPair("message", ast.NewString(msg))
	typNode := ast.NewPair("type", ast.NewAny(typ))
	node := ast.NewObject([]ast.Pair{message, typNode})
	s.notifClient(showMessage, &node)
}
