package lsp

import (
	"encoding/json"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/workspace"
)

var man = workspace.NewManager()

type Json = *ast.Node

type Role string

const (
	Client Role = "client"
	Gopls  Role = "gopls"
)

func (r Role) Revert() Role {
	switch r {
	case Client:
		return Gopls
	case Gopls:
		return Client
	}
	panic("Unknoen role")
}

type Response struct {
	Err    error
	Result json.RawMessage
}

type Request struct {
	Method string
	Params json.RawMessage
}

type Bridge interface {
	Call(role Role, call Request, callback Callback)
	Notify(role Role, notification Request) error
}

type Callback func(Response)

type Router = *router

type router struct {
	session      *session
	clientNotifs map[method]onNotif
	serverNotifs map[method]onNotif
	clientCalls  map[method]onCall
	serverCalls  map[method]onCall
}

type caller interface {
	forward()
	enc() common.Encoding
	method() method
	session() *session
	proxy(params Json, handler func(res Json))
	res(result Json)
	err(err *common.Err)
}

type notifier interface {
	forward()
	enc() common.Encoding
	method() method
	notify(params Json)
	session() *session
	err(err *common.Err)
}

type request struct {
	data json.RawMessage
	cb   Callback
	sess *session
	m    method
	role Role
}

func (c *request) method() method {
	return c.m
}

func (c *request) enc() common.Encoding {
	return c.sess.enc()
}

func (c *request) session() *session {
	return c.sess
}

func (c *request) notify(params Json) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	c.sess.bridge.Notify(c.role.Revert(), Request{
		Method: string(c.m),
		Params: data,
	})
}

func (c *request) res(result Json) {
	data, err := result.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	c.cb(Response{Result: data})
}

func (c *request) err(err *common.Err) {
	if c.isCall() {
		c.cb(Response{Err: err.Wire})
		c.sess.error(err.Error())
	} else {
		c.sess.error(err.Error())
	}
}

func (c *request) isCall() bool {
	return c.cb != nil
}

func (c *request) forward() {
	if c.isCall() {
		c.sess.bridge.Call(c.role.Revert(), Request{
			Method: string(c.m),
			Params: c.data,
		}, c.cb)
	} else {
		c.sess.bridge.Notify(c.role.Revert(), Request{
			Method: string(c.m),
			Params: c.data,
		})
	}
}

func (c *request) proxy(params Json, handler func(res Json)) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	c.sess.bridge.Call(c.role.Revert(), Request{
		Method: string(c.m),
		Params: data,
	}, func(r Response) {
		if r.Err != nil {
			c.cb(r)
			return
		}
		node, err := sonic.Get(r.Result)
		if err != nil {
			slog.Error("Unmarshal error: " + err.Error())
			c.cb(r)
			return
		}
		handler(&node)
	})
}

type onNotif func(n notifier, j Json)
type onCall func(c caller, j Json)

func (r Router) Notification(role Role, n Request) {
	slog.Info("Notification", "role", role, "method", n.Method)
	m := method(n.Method)
	switch role {
	case Client:
		if handler, ok := r.clientNotifs[m]; ok {
			node, err := sonic.Get(n.Params)
			if err != nil {
				slog.Error("Unmarshal error: " + err.Error())
				return
			}
			handler(&request{
				data: n.Params,
				sess: r.session,
				role: role,
				m:    m,
			}, &node)
			return
		}
	case Gopls:
		if handler, ok := r.serverNotifs[m]; ok {
			node, err := sonic.Get(n.Params)
			if err != nil {
				slog.Error("Unmarshal error: " + err.Error())
				return
			}
			handler(&request{
				data: n.Params,
				sess: r.session,
				role: role,
				m:    m,
			}, &node)
			return
		}
	default:
		panic("unknown role")
	}
	r.session.bridge.Notify(role.Revert(), n)
}

func (r Router) Call(role Role, call Request, cb Callback) {
	slog.Info("Call", "role", role, "method", call.Method)
	m := method(call.Method)
	switch role {
	case Client:
		if handler, ok := r.clientCalls[m]; ok {
			node, err := sonic.Get(call.Params)
			if err != nil {
				slog.Error("Unmarshal error: " + err.Error())
				return
			}
			handler(&request{
				sess: r.session,
				role: role,
				m:    m,
				data: call.Params,
				cb:   cb,
			}, &node)
			return
		}
	case Gopls:
		if handler, ok := r.serverCalls[m]; ok {
			node, err := sonic.Get(call.Params)
			if err != nil {
				slog.Error("Unmarshal error: " + err.Error())
				return
			}
			handler(&request{
				sess: r.session,
				role: role,
				data: call.Params,
				m:    m,
				cb:   cb,
			}, &node)
			return
		}
	default:
		panic("unknown role")
	}
	r.session.bridge.Call(role.Revert(), call, cb)
}

func NewRouter(bridge Bridge) Router {
	b := builder{
		clientNotifs: make(map[method]onNotif),
		serverNotifs: make(map[method]onNotif),
		clientCalls:  make(map[method]onCall),
		serverCalls:  make(map[method]onCall),
	}
	return b.build(bridge)
}

type builder struct {
	clientNotifs map[method]onNotif
	serverNotifs map[method]onNotif
	clientCalls  map[method]onCall
	serverCalls  map[method]onCall
}

func (b *builder) build(br Bridge) Router {
	initClientCalls(b.clientCall)
	initClientNotifs(b.clientNotif)
	initServerCalls(b.serverCall)
	initServerNotifs(b.serverNotif)
	return &router{
		session: &session{
			bridge: br,
		},
		clientNotifs: b.clientNotifs,
		serverNotifs: b.serverNotifs,
		clientCalls:  b.clientCalls,
		serverCalls:  b.serverCalls,
	}
}

func (b *builder) clientNotif(on onNotif, m ...method) {
	if len(m) == 0 {
		panic("no method")
	}
	for _, method := range m {
		_, ok := b.clientNotifs[method]
		if ok {
			panic("duplicate client notif handler")
		}
		b.clientNotifs[method] = on
	}
}

func (b *builder) clientCall(on onCall, m ...method) {
	if len(m) == 0 {
		panic("no method")
	}
	for _, method := range m {
		_, ok := b.clientCalls[method]
		if ok {
			panic("duplicate client call handler")
		}
		b.clientCalls[method] = on
	}
}

func (b *builder) serverNotif(on onNotif, m ...method) {
	if len(m) == 0 {
		panic("no method")
	}
	for _, method := range m {
		_, ok := b.serverNotifs[method]
		if ok {
			panic("duplicate server notif handler")
		}
		b.serverNotifs[method] = on
	}
}

func (b *builder) serverCall(on onCall, m ...method) {
	if len(m) == 0 {
		panic("no method")
	}
	for _, method := range m {
		_, ok := b.serverCalls[method]
		if ok {
			panic("duplicate server call handler")
		}
		b.serverCalls[method] = on
	}
}

type session struct {
	bridge   Bridge
	encoding common.Encoding
}

func (s session) enc() common.Encoding {
	return s.encoding
}

func (s session) setEnc(e common.Encoding) {
	s.encoding = e
}

func (s session) notifGo(m method, params Json) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	s.bridge.Notify(Gopls, Request{
		Method: string(m),
		Params: data,
	})
}

func (s session) notifClient(m method, params Json) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	s.bridge.Notify(Client, Request{
		Method: string(m),
		Params: data,
	})
}

func (s session) callClient(m method, params Json) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	s.bridge.Call(Client, Request{
		Method: string(m),
		Params: data,
	}, func(r Response) {
		slog.Info("call client result", "result", string(r.Result))
		if r.Err != nil {
			slog.Error("call client result error: " + r.Err.Error())
		}
	})
}

func (s session) info(msg string) {
	s.show(msg, 3)
}

func (s session) warn(msg string) {
	s.show(msg, 2)
}

func (s session) error(msg string) {
	s.show(msg, 1)
}

func (s session) show(msg string, typ int) {
	message := ast.NewPair("message", ast.NewString(msg))
	typNode := ast.NewPair("type", ast.NewAny(typ))
	node := ast.NewObject([]ast.Pair{message, typNode})
	s.notifClient(windowShowMessage, &node)
}
